import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/authz/permissions_provider.dart';
import '../data/asset_action_repository.dart';
import '../data/asset_dto.dart';
import 'asset_by_tag_provider.dart';
import 'asset_actions.dart';

/// Aksi FR-M7 yang SUDAH terpasang UI-nya. Bertambah per fase: M7-4 Peminjaman;
/// M7-5 Check-out & Check-in; Lapor Kerusakan (M7-6) menyusul — [assetActionsFor]
/// sudah menghitung semuanya, tapi hanya yang di sini yang dirender agar tidak
/// ada tombol tanpa aksi.
const Set<AssetAction> _implementedActions = <AssetAction>{
  AssetAction.borrow,
  AssetAction.checkout,
  AssetAction.checkin,
};

/// Bar aksi sticky di kaki Detail Aset (di luar sesi opname). Tombol muncul
/// sesuai permission pemanggil ([permissionsProvider]) x status aset. Tanpa
/// aksi -> tidak dirender (detail tetap read-only murni).
class AssetActionBar extends ConsumerWidget {
  const AssetActionBar({required this.asset, super.key});

  final AssetDto asset;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final Set<String> permissions =
        ref.watch(permissionsProvider).value ?? const <String>{};
    final List<AssetAction> actions = assetActionsFor(permissions, asset.status)
        .where(_implementedActions.contains)
        .toList();
    if (actions.isEmpty) {
      return const SizedBox.shrink();
    }

    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);

    return Material(
      color: theme.cardTheme.color ?? theme.colorScheme.surface,
      elevation: 8,
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 10, 20, 10),
          child: Row(
            children: <Widget>[
              for (final AssetAction action in actions) ...<Widget>[
                if (action != actions.first) const SizedBox(width: 10),
                Expanded(
                  child: FilledButton.icon(
                    onPressed: () => _onAction(context, ref, action),
                    icon: Icon(_actionIcon(action), size: 18),
                    label: Text(_actionLabel(l10n, action)),
                    style: FilledButton.styleFrom(
                      minimumSize: const Size(0, 48),
                    ),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  void _onAction(BuildContext context, WidgetRef ref, AssetAction action) {
    switch (action) {
      case AssetAction.borrow:
        showModalBottomSheet<void>(
          context: context,
          isScrollControlled: true,
          showDragHandle: true,
          builder: (BuildContext context) => _BorrowSheet(asset: asset),
        );
      case AssetAction.checkout:
        showModalBottomSheet<void>(
          context: context,
          isScrollControlled: true,
          showDragHandle: true,
          builder: (BuildContext context) => _CheckoutSheet(asset: asset),
        );
      case AssetAction.checkin:
        showModalBottomSheet<void>(
          context: context,
          isScrollControlled: true,
          showDragHandle: true,
          builder: (BuildContext context) => _CheckinSheet(asset: asset),
        );
      case AssetAction.reportDamage:
        // Belum dirender (lihat _implementedActions).
        break;
    }
  }
}

String _actionLabel(AppLocalizations l10n, AssetAction action) {
  return switch (action) {
    AssetAction.borrow => l10n.assetActionBorrow,
    AssetAction.checkout => l10n.assetActionCheckout,
    AssetAction.checkin => l10n.assetActionCheckin,
    AssetAction.reportDamage => l10n.assetActionReportDamage,
  };
}

IconData _actionIcon(AssetAction action) {
  return switch (action) {
    AssetAction.borrow => Symbols.handshake_rounded,
    AssetAction.checkout => Symbols.logout_rounded,
    AssetAction.checkin => Symbols.login_rounded,
    AssetAction.reportDamage => Symbols.build_rounded,
  };
}

/// Sheet Ajukan Peminjaman (Staf): jatuh tempo opsional + catatan opsional lalu
/// `POST /assignments/borrow` (pengajuan via approval). Sukses -> SnackBar; error
/// -> pesan inline.
class _BorrowSheet extends ConsumerStatefulWidget {
  const _BorrowSheet({required this.asset});

  final AssetDto asset;

  @override
  ConsumerState<_BorrowSheet> createState() => _BorrowSheetState();
}

class _BorrowSheetState extends ConsumerState<_BorrowSheet> {
  final TextEditingController _notes = TextEditingController();
  DateTime? _dueDate;
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _notes.dispose();
    super.dispose();
  }

  Future<void> _pickDueDate() async {
    final DateTime now = DateTime.now();
    final DateTime? picked = await showDatePicker(
      context: context,
      firstDate: now,
      lastDate: DateTime(now.year + 5),
      initialDate: _dueDate ?? now,
    );
    if (picked != null) {
      setState(() => _dueDate = picked);
    }
  }

  Future<void> _submit() async {
    setState(() {
      _submitting = true;
      _error = null;
    });
    final AppLocalizations l10n = AppLocalizations.of(context);
    try {
      await ref
          .read(assetActionRepositoryProvider)
          .borrow(
            assetId: widget.asset.id ?? '',
            dueDate: _dueDate == null
                ? null
                : DateFormat('yyyy-MM-dd').format(_dueDate!),
            notes: _notes.text,
          );
      if (!mounted) {
        return;
      }
      // Muat ulang detail (nama/tag tak berubah, tapi selaras pola refresh).
      final String? tag = widget.asset.assetTag;
      if (tag != null && tag.isNotEmpty) {
        ref.invalidate(assetByTagProvider(tag));
      }
      Navigator.of(context).pop();
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.borrowSuccess)));
    } on Object {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.borrowError;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final String dueLabel = _dueDate == null
        ? l10n.borrowPickDate
        : DateFormat('d MMM y', localeName).format(_dueDate!);

    return Padding(
      padding: EdgeInsets.fromLTRB(
        20,
        0,
        20,
        20 + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            l10n.borrowSheetTitle,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            widget.asset.name ?? l10n.catalogUnnamedAsset,
            style: TextStyle(color: theme.colorScheme.onSurfaceVariant),
          ),
          const SizedBox(height: 16),
          Text(l10n.borrowDueDateLabel, style: theme.textTheme.labelLarge),
          const SizedBox(height: 6),
          OutlinedButton.icon(
            onPressed: _submitting ? null : _pickDueDate,
            icon: const Icon(Symbols.calendar_month_rounded, size: 18),
            label: Text(dueLabel),
            style: OutlinedButton.styleFrom(
              alignment: Alignment.centerLeft,
              minimumSize: const Size.fromHeight(48),
            ),
          ),
          const SizedBox(height: 14),
          Text(l10n.borrowNotesLabel, style: theme.textTheme.labelLarge),
          const SizedBox(height: 6),
          TextField(
            controller: _notes,
            enabled: !_submitting,
            minLines: 2,
            maxLines: 4,
            decoration: InputDecoration(hintText: l10n.borrowNotesHint),
          ),
          const SizedBox(height: 8),
          Text(
            l10n.borrowPendingNote,
            style: TextStyle(
              fontSize: 12,
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          if (_error != null) ...<Widget>[
            const SizedBox(height: 8),
            Text(
              _error!,
              style: TextStyle(color: theme.colorScheme.error, fontSize: 13),
            ),
          ],
          const SizedBox(height: 16),
          FilledButton(
            onPressed: _submitting ? null : _submit,
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(50)),
            child: _submitting
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2.5),
                  )
                : Text(l10n.borrowSubmit),
          ),
        ],
      ),
    );
  }
}

/// Sheet Check-out (Manager): pilih custodian + tanggal + kondisi lalu
/// `POST /assignments`. Aset menjadi `assigned`.
class _CheckoutSheet extends ConsumerStatefulWidget {
  const _CheckoutSheet({required this.asset});

  final AssetDto asset;

  @override
  ConsumerState<_CheckoutSheet> createState() => _CheckoutSheetState();
}

class _CheckoutSheetState extends ConsumerState<_CheckoutSheet> {
  final TextEditingController _employeeSearch = TextEditingController();
  final TextEditingController _condition = TextEditingController();
  EmployeeOption? _employee;
  String _query = '';
  DateTime _checkoutDate = DateTime.now();
  DateTime? _dueDate;
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _employeeSearch.dispose();
    _condition.dispose();
    super.dispose();
  }

  Future<void> _pickDate({required bool checkout}) async {
    final DateTime now = DateTime.now();
    final DateTime? picked = await showDatePicker(
      context: context,
      firstDate: checkout ? DateTime(now.year - 1) : now,
      lastDate: DateTime(now.year + 5),
      initialDate: (checkout ? _checkoutDate : _dueDate) ?? now,
    );
    if (picked != null) {
      setState(() {
        if (checkout) {
          _checkoutDate = picked;
        } else {
          _dueDate = picked;
        }
      });
    }
  }

  Future<void> _submit() async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final EmployeeOption? employee = _employee;
    if (employee == null) {
      setState(() => _error = l10n.checkoutEmployeeRequired);
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await ref
          .read(assetActionRepositoryProvider)
          .checkout(
            assetId: widget.asset.id ?? '',
            employeeId: employee.id,
            checkoutDate: DateFormat('yyyy-MM-dd').format(_checkoutDate),
            dueDate: _dueDate == null
                ? null
                : DateFormat('yyyy-MM-dd').format(_dueDate!),
            conditionOut: _condition.text,
          );
      if (!mounted) {
        return;
      }
      final String? tag = widget.asset.assetTag;
      if (tag != null && tag.isNotEmpty) {
        ref.invalidate(assetByTagProvider(tag));
      }
      Navigator.of(context).pop();
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.checkoutSuccess)));
    } on Object {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.checkoutError;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;

    return Padding(
      padding: EdgeInsets.fromLTRB(
        20,
        0,
        20,
        20 + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text(
              l10n.checkoutSheetTitle,
              style: theme.textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              widget.asset.name ?? l10n.catalogUnnamedAsset,
              style: TextStyle(color: theme.colorScheme.onSurfaceVariant),
            ),
            const SizedBox(height: 16),
            Text(l10n.checkoutEmployeeLabel, style: theme.textTheme.labelLarge),
            const SizedBox(height: 6),
            if (_employee != null)
              ListTile(
                contentPadding: EdgeInsets.zero,
                leading: const Icon(Symbols.person_rounded),
                title: Text(_employee!.name),
                trailing: TextButton(
                  onPressed: _submitting
                      ? null
                      : () => setState(() => _employee = null),
                  child: Text(l10n.commonChange),
                ),
              )
            else ...<Widget>[
              TextField(
                controller: _employeeSearch,
                enabled: !_submitting,
                onChanged: (String v) => setState(() => _query = v),
                decoration: InputDecoration(
                  hintText: l10n.checkoutEmployeeSearchHint,
                  prefixIcon: const Icon(Symbols.search_rounded, size: 20),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 6),
              _EmployeeResults(
                query: _query,
                onPick: (EmployeeOption e) => setState(() => _employee = e),
              ),
            ],
            const SizedBox(height: 12),
            Text(l10n.checkoutDateLabel, style: theme.textTheme.labelLarge),
            const SizedBox(height: 6),
            OutlinedButton.icon(
              onPressed: _submitting ? null : () => _pickDate(checkout: true),
              icon: const Icon(Symbols.calendar_month_rounded, size: 18),
              label: Text(
                DateFormat('d MMM y', localeName).format(_checkoutDate),
              ),
              style: OutlinedButton.styleFrom(
                alignment: Alignment.centerLeft,
                minimumSize: const Size.fromHeight(48),
              ),
            ),
            const SizedBox(height: 12),
            Text(l10n.borrowDueDateLabel, style: theme.textTheme.labelLarge),
            const SizedBox(height: 6),
            OutlinedButton.icon(
              onPressed: _submitting ? null : () => _pickDate(checkout: false),
              icon: const Icon(Symbols.event_rounded, size: 18),
              label: Text(
                _dueDate == null
                    ? l10n.borrowPickDate
                    : DateFormat('d MMM y', localeName).format(_dueDate!),
              ),
              style: OutlinedButton.styleFrom(
                alignment: Alignment.centerLeft,
                minimumSize: const Size.fromHeight(48),
              ),
            ),
            const SizedBox(height: 12),
            Text(l10n.checkoutConditionLabel, style: theme.textTheme.labelLarge),
            const SizedBox(height: 6),
            TextField(
              controller: _condition,
              enabled: !_submitting,
              minLines: 1,
              maxLines: 3,
              decoration: const InputDecoration(isDense: true),
            ),
            const SizedBox(height: 8),
            Text(
              l10n.checkoutAssignedNote,
              style: TextStyle(
                fontSize: 12,
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            if (_error != null) ...<Widget>[
              const SizedBox(height: 8),
              Text(
                _error!,
                style: TextStyle(color: theme.colorScheme.error, fontSize: 13),
              ),
            ],
            const SizedBox(height: 16),
            FilledButton(
              onPressed: _submitting ? null : _submit,
              style: FilledButton.styleFrom(
                minimumSize: const Size.fromHeight(50),
              ),
              child: _submitting
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2.5),
                    )
                  : Text(l10n.checkoutSubmit),
            ),
          ],
        ),
      ),
    );
  }
}

/// Hasil pencarian pegawai (untuk picker check-out). Menampilkan loading,
/// "Tidak ada data", atau daftar tap-untuk-pilih.
class _EmployeeResults extends ConsumerWidget {
  const _EmployeeResults({required this.query, required this.onPick});

  final String query;
  final ValueChanged<EmployeeOption> onPick;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    return FutureBuilder<List<EmployeeOption>>(
      future: ref.read(assetActionRepositoryProvider).searchEmployees(query),
      builder:
          (BuildContext context, AsyncSnapshot<List<EmployeeOption>> snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Padding(
                padding: EdgeInsets.symmetric(vertical: 12),
                child: Center(
                  child: SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2.5),
                  ),
                ),
              );
            }
            final List<EmployeeOption> results =
                snapshot.data ?? const <EmployeeOption>[];
            if (results.isEmpty) {
              return Padding(
                padding: const EdgeInsets.symmetric(vertical: 12),
                child: Text(
                  l10n.catalogFilterNoOptions,
                  style: TextStyle(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              );
            }
            return ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 220),
              child: ListView(
                shrinkWrap: true,
                children: <Widget>[
                  for (final EmployeeOption e in results)
                    ListTile(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      leading: const Icon(Symbols.person_rounded, size: 20),
                      title: Text(e.name),
                      onTap: () => onPick(e),
                    ),
                ],
              ),
            );
          },
    );
  }
}

/// Sheet Check-in (Manager): resolusi penugasan aktif lalu kondisi masuk +
/// `POST /assignments/:id/checkin`. Aset kembali `available`/`under_maintenance`.
class _CheckinSheet extends ConsumerStatefulWidget {
  const _CheckinSheet({required this.asset});

  final AssetDto asset;

  @override
  ConsumerState<_CheckinSheet> createState() => _CheckinSheetState();
}

class _CheckinSheetState extends ConsumerState<_CheckinSheet> {
  final TextEditingController _condition = TextEditingController();
  late Future<ActiveAssignment?> _active;
  bool _needsMaintenance = false;
  bool _submitting = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _active = ref
        .read(assetActionRepositoryProvider)
        .activeAssignment(widget.asset.id ?? '');
  }

  @override
  void dispose() {
    _condition.dispose();
    super.dispose();
  }

  Future<void> _submit(ActiveAssignment active) async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await ref
          .read(assetActionRepositoryProvider)
          .checkin(
            assignmentId: active.id,
            conditionIn: _condition.text,
            needsMaintenance: _needsMaintenance,
          );
      if (!mounted) {
        return;
      }
      final String? tag = widget.asset.assetTag;
      if (tag != null && tag.isNotEmpty) {
        ref.invalidate(assetByTagProvider(tag));
      }
      Navigator.of(context).pop();
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.checkinSuccess)));
    } on Object {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.checkinError;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);

    return Padding(
      padding: EdgeInsets.fromLTRB(
        20,
        0,
        20,
        20 + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: FutureBuilder<ActiveAssignment?>(
        future: _active,
        builder:
            (BuildContext context, AsyncSnapshot<ActiveAssignment?> snapshot) {
              if (snapshot.connectionState == ConnectionState.waiting) {
                return const Padding(
                  padding: EdgeInsets.symmetric(vertical: 40),
                  child: Center(child: CircularProgressIndicator()),
                );
              }
              final ActiveAssignment? active = snapshot.data;
              if (active == null) {
                return Padding(
                  padding: const EdgeInsets.symmetric(vertical: 32),
                  child: Text(
                    l10n.checkinNoActive,
                    style: TextStyle(color: theme.colorScheme.error),
                  ),
                );
              }
              return Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(
                    l10n.checkinSheetTitle,
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    widget.asset.name ?? l10n.catalogUnnamedAsset,
                    style: TextStyle(color: theme.colorScheme.onSurfaceVariant),
                  ),
                  const SizedBox(height: 12),
                  Text(l10n.checkinHolderLabel, style: theme.textTheme.labelLarge),
                  Text(active.holderName ?? '—'),
                  const SizedBox(height: 14),
                  Text(l10n.checkinConditionLabel, style: theme.textTheme.labelLarge),
                  const SizedBox(height: 6),
                  Row(
                    children: <Widget>[
                      ChoiceChip(
                        label: Text(l10n.checkinConditionGood),
                        selected: !_needsMaintenance,
                        onSelected: _submitting
                            ? null
                            : (_) => setState(() => _needsMaintenance = false),
                      ),
                      const SizedBox(width: 8),
                      ChoiceChip(
                        label: Text(l10n.checkinConditionNeedsService),
                        selected: _needsMaintenance,
                        onSelected: _submitting
                            ? null
                            : (_) => setState(() => _needsMaintenance = true),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Text(l10n.checkinNotesLabel, style: theme.textTheme.labelLarge),
                  const SizedBox(height: 6),
                  TextField(
                    controller: _condition,
                    enabled: !_submitting,
                    minLines: 1,
                    maxLines: 3,
                    decoration: const InputDecoration(isDense: true),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    l10n.checkinReturnNote,
                    style: TextStyle(
                      fontSize: 12,
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                  if (_error != null) ...<Widget>[
                    const SizedBox(height: 8),
                    Text(
                      _error!,
                      style: TextStyle(
                        color: theme.colorScheme.error,
                        fontSize: 13,
                      ),
                    ),
                  ],
                  const SizedBox(height: 16),
                  FilledButton(
                    onPressed: _submitting ? null : () => _submit(active),
                    style: FilledButton.styleFrom(
                      minimumSize: const Size.fromHeight(50),
                    ),
                    child: _submitting
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2.5),
                          )
                        : Text(l10n.checkinSubmit),
                  ),
                ],
              );
            },
      ),
    );
  }
}
