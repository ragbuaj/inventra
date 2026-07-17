//go:build integration

// Integration tests for the sweeper against a real Postgres + Redis: the due
// scan, the retention purge, the advisory lock, and the recipient inverse.
// sweeper.go exposes an exported Tick so each test drives one sweep pass
// deterministically instead of waiting on the polling loop.
//
// Recipients here are resolved through the REAL scope service against real
// roles, permissions and offices -- the whole point of these tests is that the
// set told a schedule is due matches the set allowed to act on it, so stubbing
// that out would test nothing.
package notification_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/notification"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// sweepFixture carries the office tree and the master rows a schedule needs, so
// each test states only what it is actually about.
type sweepFixture struct {
	*harness
	tree     testsupport.OfficeTree
	category uuid.UUID
	room     uuid.UUID
	room2    uuid.UUID
	scope    *authz.ScopeService
}

// newSweepFixture resets the DB and seeds the master data every asset row needs
// (category, floor, room per branch).
func newSweepFixture(t *testing.T) *sweepFixture {
	t.Helper()
	h := newHarness(t)
	h.resetSweep(t)
	ctx := context.Background()

	tree := testsupport.SeedOfficeTree(t, h.pool)

	var category uuid.UUID
	require.NoError(t, h.pool.QueryRow(ctx,
		`INSERT INTO masterdata.categories (name, code) VALUES ('IT', 'IT') RETURNING id`).
		Scan(&category))

	floor := testsupport.SeedFloor(t, h.pool, tree.Cabang, "L1")
	floor2 := testsupport.SeedFloor(t, h.pool, tree.Cabang2, "L1")

	return &sweepFixture{
		harness:  h,
		tree:     tree,
		category: category,
		room:     testsupport.SeedRoom(t, h.pool, floor, "R1"),
		room2:    testsupport.SeedRoom(t, h.pool, floor2, "R1"),
		scope:    authz.NewScopeService(h.q, h.rdb),
	}
}

// resetSweep clears the notification pipeline plus the asset and maintenance
// schemas. testsupport.Reset only truncates identity/masterdata/audit, so the
// schedules a sweep scans would otherwise survive between tests.
func (h *harness) resetSweep(t *testing.T) {
	t.Helper()
	testsupport.Reset(t, h.pool)
	h.resetOutbox(t)
	_, err := h.pool.Exec(context.Background(),
		`TRUNCATE maintenance.maintenance_schedules, maintenance.maintenance_records,
		 asset.assets RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

// seedAsset inserts an asset in the given office and returns its id.
func (f *sweepFixture) seedAsset(t *testing.T, officeID, roomID uuid.UUID, tag string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets (asset_tag, name, category_id, office_id, room_id)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		tag, "Asset "+tag, f.category, officeID, roomID).Scan(&id))
	return id
}

// seedSchedule inserts an active schedule due dueInDays from today (negative =
// already overdue) and returns its id plus the due date as the sweeper formats it.
func (f *sweepFixture) seedSchedule(t *testing.T, assetID uuid.UUID, dueInDays int) (uuid.UUID, string) {
	t.Helper()
	due := time.Now().AddDate(0, 0, dueInDays)
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`INSERT INTO maintenance.maintenance_schedules (asset_id, interval_months, next_due_date, is_active)
		 VALUES ($1, 6, $2, true) RETURNING id`,
		assetID, due).Scan(&id))
	return id, due.Format("2006-01-02")
}

// seedManager creates a role holding maintenance.manage with the given scope
// level for the maintenance module, and a user in that role at officeID.
func (f *sweepFixture) seedManager(t *testing.T, officeID uuid.UUID, level sqlc.SharedScopeLevel, email string) uuid.UUID {
	return f.seedUserWithPerms(t, officeID, level, email, "maintenance.manage")
}

// seedUserWithPerms creates a role with exactly the given permission keys and a
// user in it, so a test can build both the entitled and the unentitled case from
// one helper.
func (f *sweepFixture) seedUserWithPerms(t *testing.T, officeID uuid.UUID, level sqlc.SharedScopeLevel, email string, perms ...string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	roleID := testsupport.SeedRole(t, f.pool, "role-"+email)
	for _, p := range perms {
		_, err := f.pool.Exec(ctx,
			`INSERT INTO identity.role_permissions (role_id, permission_key) VALUES ($1, $2)`,
			roleID, p)
		require.NoError(t, err)
	}
	testsupport.SeedScopePolicy(t, f.pool, roleID, "maintenance", level)

	var id uuid.UUID
	// name and email are separate placeholders even though they carry the same
	// value: email is citext and name is text, so one shared $1 leaves Postgres
	// unable to deduce a single type for it.
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		email, email, roleID, officeID).Scan(&id))
	return id
}

// newSweeper builds a sweeper with the given retention window.
func (f *sweepFixture) newSweeper(retentionDays int) *notification.Sweeper {
	return notification.NewSweeper(f.q, f.pool, retentionDays, time.Hour)
}

// newDueConsumer builds a consumer wired to the real scope service, with a
// ~instant min-idle so the XAUTOCLAIM path needs no waiting.
func (f *sweepFixture) newDueConsumer(name string) *notification.Consumer {
	return notification.NewConsumer(f.q, f.rdb, nil, f.scope, name, time.Second, time.Millisecond)
}

// drainToFeed runs relay then consumer once each, moving every enqueued event
// all the way into the notifications table.
func (f *sweepFixture) drainToFeed(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	relay := notification.NewRelay(f.q, f.pool, f.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)
	_, err = f.newDueConsumer("due-e2e").Tick(ctx)
	require.NoError(t, err)
}

// outboxCount reports how many live maintenance_due rows sit in the outbox.
func (f *sweepFixture) outboxCount(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM notification.outbox
		 WHERE event_type = $1 AND deleted_at IS NULL`,
		notification.EventMaintenanceDue).Scan(&n))
	return n
}

// The headline acceptance criterion, proven through the whole pipeline: a
// schedule coming due produces a real notification via outbox, relay, stream and
// consumer -- nothing mocked, so the wire contract between the sweeper and the
// fan-out is actually exercised.
func TestSweeperMaintenanceDueEndToEnd(t *testing.T) {
	f := newSweepFixture(t)

	manager := f.seedManager(t, f.tree.Cabang, sqlc.SharedScopeLevelOffice, "sweep.e2e.mgr@test.local")
	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-E2E-1")
	schedule, dueDate := f.seedSchedule(t, asset, 1)

	enqueued, err := f.newSweeper(90).Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, enqueued)

	f.drainToFeed(t)

	rows := f.notifications(t, manager)
	require.Len(t, rows, 1)
	assert.Equal(t, sqlc.SharedNotificationTypeMaintenanceDue, rows[0].Type)

	require.NotNil(t, rows[0].DedupKey)
	assert.Equal(t, fmt.Sprintf("schedule:%s:due:%s", schedule, dueDate), *rows[0].DedupKey)

	// The notification deep-links to the asset, not the schedule.
	require.NotNil(t, rows[0].EntityType)
	assert.Equal(t, "assets", *rows[0].EntityType)
	require.NotNil(t, rows[0].EntityID)
	assert.Equal(t, asset, *rows[0].EntityID)

	// Params carry interpolation values only. A rendered Indonesian sentence
	// here would freeze the notification into one locale.
	var params map[string]string
	require.NoError(t, json.Unmarshal(rows[0].Params, &params))
	assert.Equal(t, map[string]string{
		"asset_tag":  "SWP-E2E-1",
		"asset_name": "Asset SWP-E2E-1",
		"due_date":   dueDate,
	}, params)
}

// Idempotency, the criterion the hourly tick rate makes non-negotiable: 24 ticks
// a day must not mean 24 reminders. Both halves are asserted, because they fail
// independently -- uq_notif_dedup would hide a piling-up outbox behind a single
// notification row.
func TestSweeperTickIsIdempotent(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()

	manager := f.seedManager(t, f.tree.Cabang, sqlc.SharedScopeLevelOffice, "sweep.idem.mgr@test.local")
	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-IDEM-1")
	f.seedSchedule(t, asset, 1)

	sweeper := f.newSweeper(90)

	first, err := sweeper.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, first, "the first tick enqueues the reminder")

	second, err := sweeper.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, second, "the second tick must enqueue nothing")

	assert.Equal(t, 1, f.outboxCount(t), "the outbox must not pile up duplicate rows")

	f.drainToFeed(t)
	assert.Len(t, f.notifications(t, manager), 1, "two ticks, one notification")

	// A third tick after the rows are published must still add nothing: the
	// guard keys on the row's existence, not on it being unpublished.
	third, err := sweeper.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, third)
	assert.Equal(t, 1, f.outboxCount(t))
}

// An overdue schedule is swept in by the same `next_due_date <= today+1` bound
// as one due tomorrow -- and still only once.
func TestSweeperEnqueuesOverdueScheduleOnce(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()

	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-OVERDUE")
	f.seedSchedule(t, asset, -30)

	sweeper := f.newSweeper(90)
	n, err := sweeper.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	n, err = sweeper.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// The look-ahead is a boundary, and a boundary deserves a test on both sides:
// a schedule outside the window must not be announced early, while one inside it
// in the same run must -- so an empty result cannot pass by accident.
func TestSweeperSkipsScheduleBeyondLookAhead(t *testing.T) {
	f := newSweepFixture(t)

	far := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-FAR")
	f.seedSchedule(t, far, 30)
	near := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-NEAR")
	nearSchedule, _ := f.seedSchedule(t, near, 1)

	n, err := f.newSweeper(90).Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the schedule inside the look-ahead is enqueued")

	var aggregateID uuid.UUID
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT aggregate_id FROM notification.outbox WHERE event_type = $1 AND deleted_at IS NULL`,
		notification.EventMaintenanceDue).Scan(&aggregateID))
	assert.Equal(t, nearSchedule, aggregateID)
}

// An inactive schedule is not a due schedule. Asserted alongside an active one
// so the exclusion is proven rather than inferred from silence.
func TestSweeperSkipsInactiveSchedule(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()

	inactive := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-INACTIVE")
	id, _ := f.seedSchedule(t, inactive, 1)
	_, err := f.pool.Exec(ctx,
		`UPDATE maintenance.maintenance_schedules SET is_active = false WHERE id = $1`, id)
	require.NoError(t, err)

	active := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-ACTIVE")
	f.seedSchedule(t, active, 1)

	n, err := f.newSweeper(90).Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the active schedule is enqueued, the inactive one is not")
}

// Each occurrence of a recurring schedule earns its own reminder: the dedup key
// is (schedule, due date), not the schedule alone. Without the date in the key
// a schedule would be announced once and then never again.
func TestSweeperReEnqueuesAfterDueDateAdvances(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()

	manager := f.seedManager(t, f.tree.Cabang, sqlc.SharedScopeLevelOffice, "sweep.recur.mgr@test.local")
	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-RECUR")
	schedule, firstDue := f.seedSchedule(t, asset, -1)

	sweeper := f.newSweeper(90)
	n, err := sweeper.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// The maintenance is done and the schedule rolls forward to a new occurrence
	// that is itself due tomorrow.
	nextDue := time.Now().AddDate(0, 0, 1)
	_, err = f.pool.Exec(ctx,
		`UPDATE maintenance.maintenance_schedules SET next_due_date = $2 WHERE id = $1`,
		schedule, nextDue)
	require.NoError(t, err)

	n, err = sweeper.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "a new due date is a new reminder")

	f.drainToFeed(t)
	rows := f.notifications(t, manager)
	require.Len(t, rows, 2)

	keys := []string{}
	for _, r := range rows {
		require.NotNil(t, r.DedupKey)
		keys = append(keys, *r.DedupKey)
	}
	assert.ElementsMatch(t, []string{
		fmt.Sprintf("schedule:%s:due:%s", schedule, firstDue),
		fmt.Sprintf("schedule:%s:due:%s", schedule, nextDue.Format("2006-01-02")),
	}, keys)
}

// The recipient rule, stated positively and negatively in the SAME run: an
// office-scoped manager at the asset's office is told; an identical manager at
// another branch is not. The in-scope assertion is what makes the out-of-scope
// one non-vacuous -- without it, a broken pipeline would pass this test.
func TestSweeperNotifiesOnlyManagersInScope(t *testing.T) {
	f := newSweepFixture(t)

	inScope := f.seedManager(t, f.tree.Cabang, sqlc.SharedScopeLevelOffice, "sweep.in@test.local")
	outOfScope := f.seedManager(t, f.tree.Cabang2, sqlc.SharedScopeLevelOffice, "sweep.out@test.local")

	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-SCOPE")
	f.seedSchedule(t, asset, 1)

	_, err := f.newSweeper(90).Tick(context.Background())
	require.NoError(t, err)
	f.drainToFeed(t)

	assert.Len(t, f.notifications(t, inScope), 1, "the manager whose scope covers the asset IS told")
	assert.Empty(t, f.notifications(t, outOfScope), "the manager at another branch is NOT told")
}

// office_subtree resolves through the hierarchy: a manager at the parent Wilayah
// covers the Cabang beneath it. The control is a subtree manager on the OTHER
// branch, whose subtree excludes the asset.
func TestSweeperNotifiesSubtreeManagerAboveTheAsset(t *testing.T) {
	f := newSweepFixture(t)

	above := f.seedManager(t, f.tree.Wilayah, sqlc.SharedScopeLevelOfficeSubtree, "sweep.above@test.local")
	sibling := f.seedManager(t, f.tree.Wilayah2, sqlc.SharedScopeLevelOfficeSubtree, "sweep.sibling@test.local")

	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-SUBTREE")
	f.seedSchedule(t, asset, 1)

	_, err := f.newSweeper(90).Tick(context.Background())
	require.NoError(t, err)
	f.drainToFeed(t)

	assert.Len(t, f.notifications(t, above), 1, "the Wilayah above the asset's Cabang IS told")
	assert.Empty(t, f.notifications(t, sibling), "the sibling Wilayah's subtree excludes the asset")
}

// A global manager is told about any office's asset -- the allScope branch of
// the scope translation, which returns no office IDs at all and would silently
// notify nobody if it were mishandled.
func TestSweeperNotifiesGlobalManagerAnywhere(t *testing.T) {
	f := newSweepFixture(t)

	global := f.seedManager(t, f.tree.Pusat, sqlc.SharedScopeLevelGlobal, "sweep.global@test.local")

	asset := f.seedAsset(t, f.tree.Cabang2, f.room2, "SWP-GLOBAL")
	f.seedSchedule(t, asset, 1)

	_, err := f.newSweeper(90).Tick(context.Background())
	require.NoError(t, err)
	f.drainToFeed(t)

	assert.Len(t, f.notifications(t, global), 1)
}

// Permission is the first gate, independent of scope: a user sitting in the
// asset's own office with maintenance.view but not maintenance.manage cannot act
// on the schedule and is never told about it. The manager beside them proves the
// run was live.
func TestSweeperNeverNotifiesUserWithoutPermission(t *testing.T) {
	f := newSweepFixture(t)

	viewer := f.seedUserWithPerms(t, f.tree.Cabang, sqlc.SharedScopeLevelOffice,
		"sweep.viewer@test.local", "maintenance.view")
	noPerms := f.seedUserWithPerms(t, f.tree.Cabang, sqlc.SharedScopeLevelGlobal,
		"sweep.noperm@test.local")
	manager := f.seedManager(t, f.tree.Cabang, sqlc.SharedScopeLevelOffice, "sweep.perm.mgr@test.local")

	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-PERM")
	f.seedSchedule(t, asset, 1)

	_, err := f.newSweeper(90).Tick(context.Background())
	require.NoError(t, err)
	f.drainToFeed(t)

	assert.Len(t, f.notifications(t, manager), 1, "the manager IS told")
	assert.Empty(t, f.notifications(t, viewer), "maintenance.view alone earns no reminder")
	assert.Empty(t, f.notifications(t, noPerms), "no permission earns no reminder, global scope notwithstanding")
}

// The purge is the retention promise: rows past the window leave the feed and
// the unread count. A row inside the window in the same table proves the cutoff
// is a cutoff and not a truncate.
func TestSweeperPurgesNotificationsPastRetention(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()

	user := f.seedUser(t, "sweep.purge@test.local")
	old := f.seedNotificationAged(t, user, "purge:old", 120)
	fresh := f.seedNotificationAged(t, user, "purge:fresh", 10)

	before, err := f.q.CountUnreadNotifications(ctx, user)
	require.NoError(t, err)
	require.Equal(t, int64(2), before)

	_, err = f.newSweeper(90).Tick(ctx)
	require.NoError(t, err)

	rows := f.notifications(t, user)
	require.Len(t, rows, 1, "the purged row is gone from the feed")
	assert.Equal(t, fresh, rows[0].ID)

	after, err := f.q.CountUnreadNotifications(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, int64(1), after, "the purged row is gone from the unread count")

	// Soft delete, per the DATABASE.md convention: the row is still there, just
	// out of every partial index.
	var deletedAt *time.Time
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT deleted_at FROM notification.notifications WHERE id = $1`, old).Scan(&deletedAt))
	assert.NotNil(t, deletedAt, "purge soft-deletes rather than hard-deletes")
}

// The purge must be safe to run every hour forever: a second pass finds the
// already-purged row gone and leaves the survivor alone.
func TestSweeperPurgeIsIdempotent(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()

	user := f.seedUser(t, "sweep.purge.idem@test.local")
	old := f.seedNotificationAged(t, user, "purge:idem:old", 120)
	f.seedNotificationAged(t, user, "purge:idem:fresh", 10)

	sweeper := f.newSweeper(90)
	require.NoError(t, firstErr(sweeper.Tick(ctx)))

	var firstDeletedAt time.Time
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT deleted_at FROM notification.notifications WHERE id = $1`, old).Scan(&firstDeletedAt))

	require.NoError(t, firstErr(sweeper.Tick(ctx)))

	var secondDeletedAt time.Time
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT deleted_at FROM notification.notifications WHERE id = $1`, old).Scan(&secondDeletedAt))
	assert.True(t, firstDeletedAt.Equal(secondDeletedAt),
		"a second purge must not re-stamp an already-purged row")

	assert.Len(t, f.notifications(t, user), 1)
}

// Published outbox rows past the window are purged too; an unpublished one never
// is, however old -- purging it would destroy an event the relay has not yet
// delivered, which is the one thing this pipeline must never do.
func TestSweeperPurgesPublishedOutboxOnly(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()

	published := f.seedOutboxAged(t, true, 120)
	unpublished := f.seedOutboxAged(t, false, 120)
	recent := f.seedOutboxAged(t, true, 10)

	_, err := f.newSweeper(90).Tick(ctx)
	require.NoError(t, err)

	assert.True(t, f.outboxDeleted(t, published), "an old published row is purged")
	assert.False(t, f.outboxDeleted(t, unpublished), "an old UNPUBLISHED row is never purged")
	assert.False(t, f.outboxDeleted(t, recent), "a recent published row is inside the window")
}

// Retention off must not mean due-scan off: the two live in one tick but are
// separate promises.
func TestSweeperRetentionDisabledStillScansDue(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()

	user := f.seedUser(t, "sweep.noret@test.local")
	old := f.seedNotificationAged(t, user, "noret:old", 500)

	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-NORET")
	f.seedSchedule(t, asset, 1)

	n, err := f.newSweeper(0).Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the due scan still runs")

	var deletedAt *time.Time
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT deleted_at FROM notification.notifications WHERE id = $1`, old).Scan(&deletedAt))
	assert.Nil(t, deletedAt, "retention disabled purges nothing")
}

// The advisory lock is what keeps two API instances from sweeping at once.
// Driving two Ticks concurrently against one due schedule must still yield
// exactly one reminder: the lock serializes them, and the loser then finds the
// row already enqueued.
func TestSweeperAdvisoryLockPreventsConcurrentSweeps(t *testing.T) {
	f := newSweepFixture(t)

	manager := f.seedManager(t, f.tree.Cabang, sqlc.SharedScopeLevelOffice, "sweep.lock.mgr@test.local")
	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-LOCK")
	f.seedSchedule(t, asset, 1)

	const instances = 4
	results := make([]int, instances)
	errs := make([]error, instances)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < instances; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release them together, to maximise the overlap
			results[i], errs[i] = f.newSweeper(90).Tick(context.Background())
		}(i)
	}
	close(start)
	wg.Wait()

	total := 0
	for i := range results {
		require.NoError(t, errs[i], "a blocked sweep waits for the lock, it does not fail")
		total += results[i]
	}
	assert.Equal(t, 1, total, "exactly one of the concurrent sweeps enqueues the reminder")
	assert.Equal(t, 1, f.outboxCount(t))

	f.drainToFeed(t)
	assert.Len(t, f.notifications(t, manager), 1)
}

// A sweep with nothing to do is not an error, and must not enqueue anything.
func TestSweeperTickOnEmptyDatabase(t *testing.T) {
	f := newSweepFixture(t)

	n, err := f.newSweeper(90).Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, 0, f.outboxCount(t))
}

// A due schedule with nobody entitled to act on it is a no-op, not a failure:
// the event is still consumed and acked, so it cannot wedge the PEL.
func TestSweeperDueWithNoRecipientsIsAcked(t *testing.T) {
	f := newSweepFixture(t)

	asset := f.seedAsset(t, f.tree.Cabang, f.room, "SWP-NOBODY")
	f.seedSchedule(t, asset, 1)

	_, err := f.newSweeper(90).Tick(context.Background())
	require.NoError(t, err)
	f.drainToFeed(t)

	assert.Equal(t, int64(0), f.pendingCount(t), "the event is acked, not left in the PEL")
}

// Run honours ctx cancellation and stops, so graceful shutdown (Task 13) has
// something to wait on.
func TestSweeperRunStopsOnContextCancel(t *testing.T) {
	f := newSweepFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	// poll 0 exercises the default-interval guard: a zero-value ticker panics.
	go func() { notification.NewSweeper(f.q, f.pool, 90, 0).Run(ctx); close(done) }()

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

// seedNotificationAged inserts a notification created ageDays ago and returns
// its id. created_at is back-dated directly: the sweeper's cutoff is measured
// against it, and no API can produce an old row on demand.
func (f *sweepFixture) seedNotificationAged(t *testing.T, userID uuid.UUID, dedupKey string, ageDays int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`INSERT INTO notification.notifications (user_id, type, dedup_key, created_at)
		 VALUES ($1, 'maintenance_due', $2, now() - make_interval(days => $3)) RETURNING id`,
		userID, dedupKey, ageDays).Scan(&id))
	return id
}

// seedOutboxAged inserts an outbox row created ageDays ago, published or not.
func (f *sweepFixture) seedOutboxAged(t *testing.T, published bool, ageDays int) uuid.UUID {
	t.Helper()
	var publishedAt *time.Time
	if published {
		now := time.Now()
		publishedAt = &now
	}
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`INSERT INTO notification.outbox (event_type, aggregate_type, aggregate_id, published_at, created_at)
		 VALUES ('purge_probe', 'requests', gen_random_uuid(), $1, now() - make_interval(days => $2))
		 RETURNING id`,
		publishedAt, ageDays).Scan(&id))
	return id
}

// outboxDeleted reports whether the outbox row has been soft-deleted.
func (f *sweepFixture) outboxDeleted(t *testing.T, id uuid.UUID) bool {
	t.Helper()
	var deletedAt *time.Time
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT deleted_at FROM notification.outbox WHERE id = $1`, id).Scan(&deletedAt))
	return deletedAt != nil
}

// firstErr drops a Tick's count so a test that only cares about the error reads
// as one line.
func firstErr(_ int, err error) error { return err }
