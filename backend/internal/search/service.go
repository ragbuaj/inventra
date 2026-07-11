package search

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// PerGroupLimit caps items per entity group (command-palette rows).
const PerGroupLimit = 5

// MinQueryLen is the minimum trimmed rune count for a search to run.
const MinQueryLen = 2

// Gate carries one entity's resolved authorization for this caller.
type Gate struct {
	Enabled   bool
	AllScope  bool
	OfficeIDs []uuid.UUID
}

// Input is the caller-resolved search request. The handler decides gates;
// the service only orchestrates queries.
type Input struct {
	Q         string
	Assets    Gate
	Employees Gate
	Offices   Gate
	Requests  Gate
	Users     bool
}

type Service struct {
	q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

// TooShort reports whether the trimmed query is below MinQueryLen runes.
func TooShort(q string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(q)) < MinQueryLen
}

// Search runs the gated entity queries concurrently and returns non-empty
// groups in the fixed order assets, employees, offices, users, requests.
func (s *Service) Search(ctx context.Context, in Input) ([]Group, error) {
	q := strings.TrimSpace(in.Q)
	slots := make([]*Group, 5)
	eg, ctx := errgroup.WithContext(ctx)

	if in.Assets.Enabled {
		eg.Go(func() error {
			rows, err := s.q.SearchAssets(ctx, sqlc.SearchAssetsParams{
				Q: &q, AllScope: in.Assets.AllScope, OfficeIds: in.Assets.OfficeIDs, Lim: PerGroupLimit,
			})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, assetItem(r))
			}
			if len(items) > 0 {
				slots[0] = &Group{Type: "assets", Total: total, Items: items}
			}
			return nil
		})
	}
	if in.Employees.Enabled {
		eg.Go(func() error {
			rows, err := s.q.SearchEmployees(ctx, sqlc.SearchEmployeesParams{
				Q: &q, AllScope: in.Employees.AllScope, OfficeIds: in.Employees.OfficeIDs, Lim: PerGroupLimit,
			})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, employeeItem(r))
			}
			if len(items) > 0 {
				slots[1] = &Group{Type: "employees", Total: total, Items: items}
			}
			return nil
		})
	}
	if in.Offices.Enabled {
		eg.Go(func() error {
			rows, err := s.q.SearchOffices(ctx, sqlc.SearchOfficesParams{
				Q: &q, AllScope: in.Offices.AllScope, OfficeIds: in.Offices.OfficeIDs, Lim: PerGroupLimit,
			})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, officeItem(r))
			}
			if len(items) > 0 {
				slots[2] = &Group{Type: "offices", Total: total, Items: items}
			}
			return nil
		})
	}
	if in.Users {
		eg.Go(func() error {
			rows, err := s.q.SearchUsers(ctx, sqlc.SearchUsersParams{Q: &q, Lim: PerGroupLimit})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, userItem(r))
			}
			if len(items) > 0 {
				slots[3] = &Group{Type: "users", Total: total, Items: items}
			}
			return nil
		})
	}
	if in.Requests.Enabled {
		eg.Go(func() error {
			rows, err := s.q.SearchRequests(ctx, sqlc.SearchRequestsParams{
				Q: &q, AllScope: in.Requests.AllScope, OfficeIds: in.Requests.OfficeIDs, Lim: PerGroupLimit,
			})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, requestItem(r))
			}
			if len(items) > 0 {
				slots[4] = &Group{Type: "requests", Total: total, Items: items}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	out := make([]Group, 0, 5)
	for _, g := range slots {
		if g != nil {
			out = append(out, *g)
		}
	}
	return out, nil
}
