import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/api/app_failure.dart';
import '../../../core/auth/auth_controller.dart';
import '../../../core/auth/auth_session.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/i18n/request_type_label.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/confirm_dialog.dart';
import '../../../core/widgets/empty_state.dart';
import '../../../core/widgets/status_chip.dart';
import '../data/approval_repository.dart';
import '../data/request_detail_dto.dart';
import '../data/request_step_dto.dart';
import 'approval_detail_provider.dart';
import 'approval_inbox_controller.dart';
import 'inbox_count_provider.dart';
import 'request_presentation.dart';

/// Nilai kosong: field null, dimask, atau nama referensi belum ter-resolve.
const String _emDash = '—';

/// Layar Detail Approval 1:1 mockup "Inventra Mobile - Detail Approval":
/// header jenis + status + maker, card "Data yang diajukan" (payload per jenis,
/// referensi di-resolve ke NAMA — UUID tidak pernah tampil), card "Jenjang
/// persetujuan" (steps kontrak), lalu kaki aksi.
///
/// Kaki aksi mengikuti field kontrak pada respons detail — klien tidak
/// menebak aturan: `status != pending` menampilkan banner status (tanpa
/// aksi); `requested_by_id == pengguna` menampilkan banner SoD (maker tidak
/// boleh memutus pengajuannya sendiri); selain itu field catatan + Tolak +
/// Setujui. 403/409 dari server saat memutus dirender sopan via SnackBar i18n.
class ApprovalDetailScreen extends ConsumerStatefulWidget {
  const ApprovalDetailScreen({required this.requestId, super.key});

  final String requestId;

  @override
  ConsumerState<ApprovalDetailScreen> createState() =>
      _ApprovalDetailScreenState();
}

class _ApprovalDetailScreenState extends ConsumerState<ApprovalDetailScreen> {
  final TextEditingController _noteController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _noteController.dispose();
    super.dispose();
  }

  Future<void> _decide({
    required RequestDetailDto request,
    required bool approve,
  }) async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String title = requestTitle(l10n, request.type, request.reason);
    final String maker = request.requestedByName ?? _emDash;
    final String note = _noteController.text.trim();

    final bool confirmed = approve
        ? await ConfirmDialog.show(
            context,
            title: l10n.approvalDetailApproveConfirmTitle,
            message: l10n.approvalDetailApproveConfirmBody(title, maker),
            confirmLabel: l10n.approvalDetailApproveConfirmAction,
            icon: Symbols.check_circle_rounded,
          )
        : await _RejectConfirmDialog.show(
            context,
            title: l10n.approvalDetailRejectConfirmTitle,
            message: l10n.approvalDetailRejectConfirmBody(title, maker),
            confirmLabel: l10n.approvalDetailRejectConfirmAction,
            noteLabel: l10n.approvalDetailYourNote,
            note: note.isEmpty ? null : note,
          );
    if (!confirmed || !mounted) {
      return;
    }

    final ScaffoldMessengerState messenger = ScaffoldMessenger.of(context);
    final NavigatorState navigator = Navigator.of(context);
    final ApprovalRepository repository = ref.read(approvalRepositoryProvider);

    setState(() => _submitting = true);
    try {
      if (approve) {
        await repository.approve(widget.requestId, note: note);
      } else {
        await repository.reject(widget.requestId, note: note);
      }
      // Keputusan mengubah isi inbox (semua filter) dan badge — segarkan
      // sebelum kembali supaya daftar yang tampil sudah mutakhir.
      ref.invalidate(approvalInboxProvider);
      ref.invalidate(approvalInboxCountProvider);
      messenger.showSnackBar(
        SnackBar(
          content: Text(
            approve
                ? l10n.approvalDetailApprovedSnack
                : l10n.approvalDetailRejectedSnack,
          ),
        ),
      );
      if (navigator.canPop()) {
        navigator.pop();
      }
    } on AppFailure catch (failure) {
      messenger.showSnackBar(
        SnackBar(content: Text(_decisionErrorMessage(l10n, failure))),
      );
      if (failure is ConflictFailure || failure is ForbiddenFailure) {
        // Status/eligibility sudah berubah di server — muat ulang detail
        // supaya kaki aksi mencerminkan keadaan terbaru.
        ref.invalidate(approvalDetailProvider(widget.requestId));
      }
    } finally {
      if (mounted) {
        setState(() => _submitting = false);
      }
    }
  }

  String _decisionErrorMessage(AppLocalizations l10n, AppFailure failure) {
    return switch (failure) {
      ForbiddenFailure() => l10n.approvalDetailErrorSod,
      ConflictFailure() => l10n.approvalDetailErrorConflict,
      NetworkFailure() => l10n.approvalDetailErrorNetwork,
      _ => l10n.approvalDetailErrorGeneric,
    };
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<ApprovalDetailData> state = ref.watch(
      approvalDetailProvider(widget.requestId),
    );
    final ApprovalReferenceNames? names = ref
        .watch(approvalReferenceNamesProvider(widget.requestId))
        .value;
    final AuthSession? session = ref.watch(authControllerProvider).value;
    final String? currentUserId = session is Authenticated
        ? session.user.id
        : null;

    return Scaffold(
      appBar: AppBar(title: Text(l10n.approvalDetailTitle)),
      body: SafeArea(
        child: state.when(
          data: (ApprovalDetailData data) => Column(
            children: <Widget>[
              Expanded(
                child: _DetailBody(data: data, names: names),
              ),
              _DecisionFooter(
                data: data,
                currentUserId: currentUserId,
                noteController: _noteController,
                submitting: _submitting,
                onApprove: () => _decide(request: data.request, approve: true),
                onReject: () => _decide(request: data.request, approve: false),
              ),
            ],
          ),
          loading: () => const _LoadingSkeleton(),
          error: (Object error, StackTrace stackTrace) => _ErrorState(
            failure: error,
            onRetry: () =>
                ref.invalidate(approvalDetailProvider(widget.requestId)),
          ),
        ),
      ),
    );
  }
}

/// Empat cabang error: 404, 403, offline, dan generik.
class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.failure, required this.onRetry});

  final Object failure;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return switch (failure) {
      NotFoundFailure() => EmptyState(
        icon: Symbols.question_mark_rounded,
        title: l10n.approvalDetailNotFoundTitle,
        subtitle: l10n.approvalDetailNotFoundBody,
      ),
      ForbiddenFailure() => EmptyState(
        icon: Symbols.lock_rounded,
        title: l10n.approvalDetailForbiddenTitle,
        subtitle: l10n.approvalDetailForbiddenBody,
      ),
      NetworkFailure() => EmptyState(
        icon: Symbols.wifi_off_rounded,
        title: l10n.approvalDetailErrorTitle,
        subtitle: l10n.approvalDetailErrorNetwork,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.approvalDetailErrorTitle,
        subtitle: l10n.approvalDetailErrorGeneric,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}

/// Skeleton loading menyusun bentuk layar: header, dua card, kaki aksi.
class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 4, 20, 24),
      children: const <Widget>[
        AppSkeleton(height: 24, width: 200, borderRadius: 999),
        SizedBox(height: 10),
        AppSkeleton(height: 20, width: 280, borderRadius: 8),
        SizedBox(height: 10),
        AppSkeleton(height: 38, borderRadius: 12),
        SizedBox(height: 14),
        AppSkeleton(height: 190, borderRadius: 18),
        SizedBox(height: 12),
        AppSkeleton(height: 170, borderRadius: 18),
        SizedBox(height: 12),
        AppSkeleton(height: 52, borderRadius: 14),
      ],
    );
  }
}

class _DetailBody extends StatelessWidget {
  const _DetailBody({required this.data, required this.names});

  final ApprovalDetailData data;
  final ApprovalReferenceNames? names;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final RequestDetailDto request = data.request;
    final bool sensitive = isSensitiveRequestType(request.type);

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 4, 20, 16),
      children: <Widget>[
        if (sensitive && request.status == 'pending') ...<Widget>[
          const _SensitiveBanner(),
          const SizedBox(height: 12),
        ],
        _Header(data: data),
        const SizedBox(height: 12),
        _SubmittedDataCard(data: data, names: names),
        const SizedBox(height: 12),
        _StepsCard(request: request),
        if (request.decisionNote != null &&
            request.decisionNote!.trim().isNotEmpty) ...<Widget>[
          const SizedBox(height: 12),
          _DecisionNoteCard(
            label: l10n.approvalDetailYourNote,
            note: request.decisionNote!,
          ),
        ],
      ],
    );
  }
}

/// Banner amber "Tindakan sensitif" (mockup state penghapusan).
class _SensitiveBanner extends StatelessWidget {
  const _SensitiveBanner();

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final StatusColorSet warning = statusColorSetOf(
      context,
      StatusChipVariant.warning,
    );

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 10),
      decoration: BoxDecoration(
        color: warning.bg,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: <Widget>[
          Icon(Symbols.warning_rounded, size: 19, color: warning.text),
          const SizedBox(width: 9),
          Expanded(
            child: Text(
              l10n.approvalDetailSensitiveBanner,
              style: TextStyle(
                fontSize: 12.5,
                fontWeight: FontWeight.w600,
                color: warning.text,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Header: chip jenis + penanda sensitif + chip status, judul, baris maker
/// (avatar inisial, nama, peran · kantor, tanggal).
class _Header extends StatelessWidget {
  const _Header({required this.data});

  final ApprovalDetailData data;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final RequestDetailDto request = data.request;
    final StatusColorSet typeColors = statusColorSetOf(
      context,
      requestTypeVariant(request.type),
    );
    final (String statusLabel, StatusChipVariant statusVariant) =
        requestStatusPresentation(l10n, request.status);
    final String makerName = request.requestedByName ?? _emDash;
    final String makerLine = <String>[
      request.requestedByRole ?? _emDash,
      if (request.officeName != null) request.officeName!,
    ].join(' · ');
    final DateTime? createdAt = request.createdAt;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Wrap(
          spacing: 8,
          runSpacing: 6,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: <Widget>[
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 4),
              decoration: ShapeDecoration(
                color: typeColors.bg,
                shape: const StadiumBorder(),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  Icon(
                    requestTypeIcon(request.type),
                    size: 14,
                    color: typeColors.text,
                  ),
                  const SizedBox(width: 5),
                  Text(
                    requestTypeLabel(l10n, request.type),
                    style: TextStyle(
                      fontSize: 11.5,
                      fontWeight: FontWeight.w700,
                      color: typeColors.text,
                    ),
                  ),
                ],
              ),
            ),
            if (isSensitiveRequestType(request.type))
              _SensitiveMarker(label: l10n.approvalCardSensitive),
            StatusChip(label: statusLabel, variant: statusVariant),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          requestTitle(l10n, request.type, request.reason),
          style: TextStyle(
            fontSize: 18,
            fontWeight: FontWeight.w800,
            letterSpacing: 18 * InventraDimens.titleLetterSpacingEm,
            color: scheme.onSurface,
          ),
        ),
        const SizedBox(height: 10),
        Row(
          children: <Widget>[
            Container(
              width: 38,
              height: 38,
              decoration: BoxDecoration(
                color: scheme.primaryContainer,
                border: Border.all(color: scheme.outlineVariant, width: 1.5),
                shape: BoxShape.circle,
              ),
              alignment: Alignment.center,
              child: Text(
                _initials(makerName),
                style: TextStyle(
                  fontSize: 12.5,
                  fontWeight: FontWeight.w700,
                  color: scheme.onPrimaryContainer,
                ),
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(
                    makerName,
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: scheme.onSurface,
                    ),
                  ),
                  Text(
                    makerLine,
                    style: TextStyle(
                      fontSize: 11.5,
                      color: theme.textTheme.bodySmall?.color,
                    ),
                  ),
                ],
              ),
            ),
            if (createdAt != null)
              Text(
                DateFormat('d MMM y', localeName).format(createdAt.toLocal()),
                style: TextStyle(
                  fontSize: 11.5,
                  color: theme.textTheme.labelSmall?.color,
                ),
              ),
          ],
        ),
      ],
    );
  }

  String _initials(String name) {
    final List<String> parts = name
        .trim()
        .split(RegExp(r'\s+'))
        .where((String part) => part.isNotEmpty)
        .toList();
    if (parts.isEmpty || parts.first == _emDash) {
      return '?';
    }
    final String first = parts.first[0];
    final String second = parts.length > 1 ? parts[1][0] : '';
    return (first + second).toUpperCase();
  }
}

/// Penanda "sensitif" kecil (titik amber + label).
class _SensitiveMarker extends StatelessWidget {
  const _SensitiveMarker({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final StatusColorSet warning = statusColorSetOf(
      context,
      StatusChipVariant.warning,
    );

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        Container(
          width: 7,
          height: 7,
          decoration: BoxDecoration(color: warning.dot, shape: BoxShape.circle),
        ),
        const SizedBox(width: 4),
        Text(
          label,
          style: TextStyle(
            fontSize: 10.5,
            fontWeight: FontWeight.w600,
            color: warning.text,
          ),
        ),
      ],
    );
  }
}

/// Satu baris data payload siap render.
@immutable
class _PayloadRow {
  const _PayloadRow({required this.label, this.value, this.change});

  final String label;

  /// Nilai tunggal; null berarti em-dash.
  final String? value;

  /// Perubahan "dari -> ke" (baris kantor mutasi); label pakai panah.
  final (String from, String to)? change;
}

/// Card "Data yang diajukan": baris payload per jenis pengajuan; referensi
/// (kantor/ruangan/kategori/dst.) di-resolve ke NAMA lewat lookup master data
/// non-fatal — UUID mentah tidak pernah ditampilkan. Payload yang dimask
/// field permission dirender penanda "dibatasi".
class _SubmittedDataCard extends StatelessWidget {
  const _SubmittedDataCard({required this.data, required this.names});

  final ApprovalDetailData data;
  final ApprovalReferenceNames? names;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final bool restricted = data.isMasked('payload') || data.isMasked('amount');
    final List<_PayloadRow> rows = _buildRows(l10n, localeName);

    return _SectionCard(
      title: l10n.approvalDetailSectionData,
      badge: restricted
          ? _RestrictedBadge(label: l10n.approvalDetailRestrictedData)
          : null,
      child: rows.isEmpty
          ? _RestrictedRow(
              label: l10n.approvalDetailRestrictedData,
              show: restricted,
            )
          : Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: <Widget>[
                for (int i = 0; i < rows.length; i++) ...<Widget>[
                  if (i > 0) ...<Widget>[
                    const SizedBox(height: 10),
                    const Divider(),
                    const SizedBox(height: 10),
                  ],
                  _PayloadRowView(row: rows[i]),
                ],
              ],
            ),
    );
  }

  List<_PayloadRow> _buildRows(AppLocalizations l10n, String localeName) {
    final RequestDetailDto request = data.request;
    final Map<String, dynamic> payload =
        request.payload ?? const <String, dynamic>{};
    final List<_PayloadRow> rows = <_PayloadRow>[];

    String? text(String key) {
      final Object? value = payload[key];
      return value is String && value.trim().isNotEmpty ? value.trim() : null;
    }

    String? resolved(String key) =>
        payload[key] == null ? null : (names?[key] ?? _emDash);

    String? money(String key) {
      final String? raw = text(key);
      return raw == null ? null : formatIdrAmount(raw, localeName);
    }

    String? date(String key) {
      final String? raw = text(key);
      return raw == null ? null : formatShortDate(raw, localeName);
    }

    void add(String label, String? value) {
      if (value != null) {
        rows.add(_PayloadRow(label: label, value: value));
      }
    }

    // Target aset (mutasi/penghapusan/peminjaman/perbaikan/pengecualian).
    if (request.targetEntity == 'assets' && request.targetId != null) {
      rows.add(
        _PayloadRow(
          label: l10n.approvalDetailFieldAsset,
          value: names?.targetLabel ?? _emDash,
        ),
      );
    }

    switch (request.type) {
      case 'asset_transfer':
        final String? from = resolved('from_office_id');
        final String? to = resolved('to_office_id');
        if (from != null || to != null) {
          rows.add(
            _PayloadRow(
              label: l10n.approvalDetailFieldOfficeChange,
              change: (from ?? _emDash, to ?? _emDash),
            ),
          );
        }
        add(l10n.approvalDetailFieldRoom, resolved('to_room_id'));
        add(l10n.approvalDetailFieldConditionSent, text('condition_sent'));
        add(l10n.approvalDetailFieldTransferDate, date('transfer_date'));
      case 'asset_create':
        add(l10n.approvalDetailFieldName, text('name'));
        add(l10n.approvalDetailFieldCategory, resolved('category_id'));
        add(l10n.approvalDetailFieldOffice, resolved('office_id'));
        add(l10n.approvalDetailFieldRoom, resolved('room_id'));
        add(l10n.approvalDetailFieldAssetClass, switch (text('asset_class')) {
          'tangible' => l10n.approvalDetailAssetClassTangible,
          'intangible' => l10n.approvalDetailAssetClassIntangible,
          final String? other => other,
        });
        add(l10n.approvalDetailFieldPurchaseCost, money('purchase_cost'));
        add(l10n.approvalDetailFieldPurchaseDate, date('purchase_date'));
        add(l10n.approvalDetailFieldSerial, text('serial_number'));
        final List<String> brandModel = <String>[
          ?resolved('brand_id'),
          ?resolved('model_id'),
        ];
        if (brandModel.isNotEmpty) {
          add(l10n.approvalDetailFieldBrandModel, brandModel.join(' · '));
        }
        add(l10n.approvalDetailFieldVendor, resolved('vendor_id'));
        add(l10n.approvalDetailFieldPoNumber, text('po_number'));
        add(l10n.approvalDetailFieldFundingSource, text('funding_source'));
        add(l10n.approvalDetailFieldWarrantyExpiry, date('warranty_expiry'));
        add(l10n.approvalDetailFieldNotes, text('notes'));
      case 'asset_disposal':
        add(l10n.approvalDetailFieldMethod, switch (text('method')) {
          'sale' => l10n.approvalDetailMethodSale,
          'auction' => l10n.approvalDetailMethodAuction,
          'donation' => l10n.approvalDetailMethodDonation,
          'write_off' => l10n.approvalDetailMethodWriteOff,
          final String? other => other,
        });
        add(l10n.approvalDetailFieldDisposalDate, date('disposal_date'));
        add(l10n.approvalDetailFieldBookValue, money('book_value_at_disposal'));
        add(l10n.approvalDetailFieldProceeds, money('proceeds'));
        add(l10n.approvalDetailFieldBastNo, text('bast_no'));
      default:
        break;
    }

    // Nominal + alasan level Request (berlaku semua jenis; absen bila dimask).
    final String? amount = request.amount;
    if (amount != null) {
      rows.add(
        _PayloadRow(
          label: l10n.approvalDetailFieldAmount,
          value: formatIdrAmount(amount, localeName),
        ),
      );
    }
    final String? reason = request.reason?.trim();
    final String? payloadReason = text('reason');
    final String? reasonValue = payloadReason ?? reason;
    if (reasonValue != null && reasonValue.isNotEmpty) {
      rows.add(
        _PayloadRow(label: l10n.approvalDetailFieldReason, value: reasonValue),
      );
    }
    return rows;
  }
}

/// Baris label kecil + nilai; varian perubahan merender "lama -> baru".
class _PayloadRowView extends StatelessWidget {
  const _PayloadRowView({required this.row});

  final _PayloadRow row;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final (String, String)? change = row.change;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text(
          row.label,
          style: TextStyle(
            fontSize: 11,
            color: theme.textTheme.labelSmall?.color,
          ),
        ),
        const SizedBox(height: 3),
        if (change != null)
          Wrap(
            spacing: 8,
            runSpacing: 4,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: <Widget>[
              Text(
                change.$1,
                style: TextStyle(
                  fontSize: 13,
                  color: theme.textTheme.labelSmall?.color,
                  decoration: TextDecoration.lineThrough,
                  decorationColor: theme.textTheme.labelSmall?.color,
                ),
              ),
              Icon(
                Symbols.arrow_forward_rounded,
                size: 15,
                color: theme.textTheme.bodySmall?.color,
              ),
              Text(
                change.$2,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w700,
                  color: scheme.onPrimaryContainer,
                ),
              ),
            ],
          )
        else
          Text(
            row.value ?? _emDash,
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              height: 1.5,
              color: scheme.onSurface,
            ),
          ),
      ],
    );
  }
}

/// Isi card data saat tidak ada baris: gembok + em-dash bila dimask, atau
/// em-dash saja.
class _RestrictedRow extends StatelessWidget {
  const _RestrictedRow({required this.label, required this.show});

  final String label;
  final bool show;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final Color mutedColor =
        theme.textTheme.labelSmall?.color ?? theme.colorScheme.onSurfaceVariant;

    if (!show) {
      return Align(
        alignment: Alignment.centerLeft,
        child: Text(_emDash, style: TextStyle(fontSize: 13, color: mutedColor)),
      );
    }
    return Tooltip(
      message: label,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          Icon(Symbols.lock_rounded, size: 15, color: mutedColor),
          const SizedBox(width: 6),
          Text(_emDash, style: TextStyle(fontSize: 13, color: mutedColor)),
        ],
      ),
    );
  }
}

/// Card "Jenjang persetujuan": maker lalu tiap step kontrak, dengan indikator
/// selesai/aktif/berikutnya (mockup timeline).
class _StepsCard extends StatelessWidget {
  const _StepsCard({required this.request});

  final RequestDetailDto request;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final List<RequestStepDto> steps = request.steps;
    final DateTime? createdAt = request.createdAt;

    String stamp(DateTime? time) => time == null
        ? _emDash
        : DateFormat('d MMM, HH.mm', localeName).format(time.toLocal());

    final List<Widget> entries = <Widget>[
      _StepRow(
        state: _StepVisual.done,
        title: request.requestedByName ?? _emDash,
        role: l10n.approvalDetailStepMaker,
        subtitle: l10n.approvalDetailStepSubmitted(stamp(createdAt)),
        subtitleEmphasis: _StepEmphasis.muted,
        isLast: steps.isEmpty,
      ),
    ];
    for (int i = 0; i < steps.length; i++) {
      final RequestStepDto step = steps[i];
      final bool isCurrent =
          request.status == 'pending' && step.stepOrder == request.currentStep;
      final (
        _StepVisual visual,
        String subtitle,
        _StepEmphasis emphasis,
      ) = switch (step.decision) {
        'approved' => (
          _StepVisual.done,
          l10n.approvalDetailStepApproved(stamp(step.decidedAt)),
          _StepEmphasis.success,
        ),
        'rejected' => (
          _StepVisual.rejected,
          l10n.approvalDetailStepRejected(stamp(step.decidedAt)),
          _StepEmphasis.danger,
        ),
        _ when isCurrent => (
          _StepVisual.active,
          l10n.approvalDetailStepWaiting,
          _StepEmphasis.warning,
        ),
        _ => (
          _StepVisual.upcoming,
          l10n.approvalDetailStepUpcoming,
          _StepEmphasis.muted,
        ),
      };
      entries.add(
        _StepRow(
          state: visual,
          title: step.approverName ?? _levelLabel(l10n, step.requiredLevel),
          role: step.approverName == null
              ? null
              : _levelLabel(l10n, step.requiredLevel),
          subtitle: subtitle,
          subtitleEmphasis: emphasis,
          note: step.note,
          isLast: i == steps.length - 1,
        ),
      );
    }

    return _SectionCard(
      title: l10n.approvalDetailSectionSteps,
      child: Column(children: entries),
    );
  }

  String _levelLabel(AppLocalizations l10n, String? level) {
    return switch (level) {
      null => _emDash,
      'office' => l10n.approvalDetailLevelOffice,
      'office_subtree' => l10n.approvalDetailLevelOfficeSubtree,
      'wilayah' => l10n.approvalDetailLevelWilayah,
      'pusat' => l10n.approvalDetailLevelPusat,
      final String other => other,
    };
  }
}

enum _StepVisual { done, rejected, active, upcoming }

enum _StepEmphasis { success, danger, warning, muted }

/// Satu baris timeline: indikator lingkaran + garis penghubung + teks.
class _StepRow extends StatelessWidget {
  const _StepRow({
    required this.state,
    required this.title,
    required this.subtitle,
    required this.subtitleEmphasis,
    required this.isLast,
    this.role,
    this.note,
  });

  final _StepVisual state;
  final String title;
  final String? role;
  final String subtitle;
  final _StepEmphasis subtitleEmphasis;
  final String? note;
  final bool isLast;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final StatusColorSet warning = statusColorSetOf(
      context,
      StatusChipVariant.warning,
    );
    final StatusColorSet success = statusColorSetOf(
      context,
      StatusChipVariant.success,
    );
    final StatusColorSet danger = statusColorSetOf(
      context,
      StatusChipVariant.danger,
    );
    final String? roleText = role;
    final String? noteText = note?.trim();

    final Widget indicator = switch (state) {
      _StepVisual.done => Container(
        width: 26,
        height: 26,
        decoration: BoxDecoration(
          color: scheme.primary,
          shape: BoxShape.circle,
        ),
        child: Icon(Symbols.check_rounded, size: 15, color: scheme.onPrimary),
      ),
      _StepVisual.rejected => Container(
        width: 26,
        height: 26,
        decoration: BoxDecoration(color: scheme.error, shape: BoxShape.circle),
        child: Icon(Symbols.close_rounded, size: 15, color: scheme.onError),
      ),
      _StepVisual.active => Container(
        width: 26,
        height: 26,
        decoration: BoxDecoration(
          color: theme.cardTheme.color ?? scheme.surface,
          shape: BoxShape.circle,
          border: Border.all(color: warning.dot, width: 2.5),
        ),
        alignment: Alignment.center,
        child: Container(
          width: 8,
          height: 8,
          decoration: BoxDecoration(color: warning.dot, shape: BoxShape.circle),
        ),
      ),
      _StepVisual.upcoming => Container(
        width: 26,
        height: 26,
        decoration: BoxDecoration(
          color: scheme.secondaryContainer,
          shape: BoxShape.circle,
          border: Border.all(color: scheme.outlineVariant, width: 2),
        ),
      ),
    };

    final Color subtitleColor = switch (subtitleEmphasis) {
      _StepEmphasis.success => success.text,
      _StepEmphasis.danger => danger.text,
      _StepEmphasis.warning => warning.text,
      _StepEmphasis.muted =>
        theme.textTheme.labelSmall?.color ?? scheme.onSurfaceVariant,
    };
    final bool muted = state == _StepVisual.upcoming;

    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: <Widget>[
          Column(
            children: <Widget>[
              indicator,
              if (!isLast)
                Expanded(
                  child: Container(
                    width: 2,
                    margin: const EdgeInsets.symmetric(vertical: 2),
                    color: state == _StepVisual.done
                        ? statusColorSetOf(
                            context,
                            StatusChipVariant.success,
                          ).bg
                        : scheme.outlineVariant,
                  ),
                ),
            ],
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Padding(
              padding: EdgeInsets.only(bottom: isLast ? 0 : 14),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text.rich(
                    TextSpan(
                      text: title,
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: muted ? FontWeight.w600 : FontWeight.w700,
                        color: muted
                            ? theme.textTheme.labelSmall?.color
                            : scheme.onSurface,
                      ),
                      children: <InlineSpan>[
                        if (roleText != null)
                          TextSpan(
                            text: ' · $roleText',
                            style: TextStyle(
                              fontWeight: FontWeight.w500,
                              color: theme.textTheme.bodySmall?.color,
                            ),
                          ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    subtitle,
                    style: TextStyle(
                      fontSize: 11.5,
                      fontWeight: subtitleEmphasis == _StepEmphasis.muted
                          ? FontWeight.w400
                          : FontWeight.w600,
                      color: subtitleColor,
                    ),
                  ),
                  if (noteText != null && noteText.isNotEmpty) ...<Widget>[
                    const SizedBox(height: 2),
                    Text(
                      '"$noteText"',
                      style: TextStyle(
                        fontSize: 11.5,
                        color: theme.textTheme.bodySmall?.color,
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Card catatan keputusan akhir (mockup "Catatan Anda" setelah diputus).
class _DecisionNoteCard extends StatelessWidget {
  const _DecisionNoteCard({required this.label, required this.note});

  final String label;
  final String note;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 11),
      decoration: BoxDecoration(
        color: scheme.secondaryContainer,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Icon(
            Symbols.sticky_note_2_rounded,
            size: 17,
            color: theme.textTheme.bodySmall?.color,
          ),
          const SizedBox(width: 9),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 11,
                    color: theme.textTheme.labelSmall?.color,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  '"$note"',
                  style: TextStyle(fontSize: 12.5, color: scheme.onSurface),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Card seksi dengan judul uppercase kecil + badge opsional.
class _SectionCard extends StatelessWidget {
  const _SectionCard({required this.title, required this.child, this.badge});

  final String title;
  final Widget child;
  final Widget? badge;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final Widget? badgeWidget = badge;

    return Container(
      padding: const EdgeInsets.fromLTRB(16, 15, 16, 15),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: <Widget>[
              Flexible(
                child: Text(
                  title.toUpperCase(),
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 12 * 0.05,
                    color: theme.textTheme.bodySmall?.color,
                  ),
                ),
              ),
              ?badgeWidget,
            ],
          ),
          const SizedBox(height: 11),
          child,
        ],
      ),
    );
  }
}

/// Badge pill "Dibatasi untuk peran Anda".
class _RestrictedBadge extends StatelessWidget {
  const _RestrictedBadge({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: ShapeDecoration(
        color: scheme.secondaryContainer,
        shape: const StadiumBorder(),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          Icon(
            Symbols.lock_rounded,
            size: 13,
            color: theme.textTheme.bodySmall?.color,
          ),
          const SizedBox(width: 5),
          Text(
            label,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: theme.textTheme.bodySmall?.color,
            ),
          ),
        ],
      ),
    );
  }
}

/// Kaki layar: field catatan + Tolak/Setujui saat masih boleh memutus; banner
/// SoD saat pengguna adalah maker; banner status saat sudah diputus.
class _DecisionFooter extends StatelessWidget {
  const _DecisionFooter({
    required this.data,
    required this.currentUserId,
    required this.noteController,
    required this.submitting,
    required this.onApprove,
    required this.onReject,
  });

  final ApprovalDetailData data;
  final String? currentUserId;
  final TextEditingController noteController;
  final bool submitting;
  final VoidCallback onApprove;
  final VoidCallback onReject;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final RequestDetailDto request = data.request;

    final Widget content;
    if (request.status != 'pending') {
      final bool byMe =
          currentUserId != null && request.decidedById == currentUserId;
      final (
        String label,
        StatusChipVariant variant,
        IconData icon,
      ) = switch (request.status) {
        'approved' => (
          byMe
              ? l10n.approvalDetailDecidedByYouApproved
              : l10n.approvalDetailDecidedApproved,
          StatusChipVariant.success,
          Symbols.check_circle_rounded,
        ),
        'rejected' => (
          byMe
              ? l10n.approvalDetailDecidedByYouRejected
              : l10n.approvalDetailDecidedRejected,
          StatusChipVariant.danger,
          Symbols.cancel_rounded,
        ),
        _ => (
          l10n.approvalDetailDecidedCancelled,
          StatusChipVariant.neutral,
          Symbols.block_rounded,
        ),
      };
      content = _FooterBanner(label: label, variant: variant, icon: icon);
    } else if (currentUserId != null &&
        request.requestedById == currentUserId) {
      // SoD dari field kontrak `requested_by_id`: maker tidak boleh memutus
      // pengajuannya sendiri — server menolak dengan 403, klien tidak
      // menawarkan aksinya sama sekali.
      content = _FooterBanner(
        label: l10n.approvalDetailSodOwnRequest,
        variant: StatusChipVariant.warning,
        icon: Symbols.info_rounded,
      );
    } else {
      content = Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          TextField(
            controller: noteController,
            enabled: !submitting,
            textInputAction: TextInputAction.done,
            decoration: InputDecoration(hintText: l10n.approvalDetailNoteHint),
            style: TextStyle(fontSize: 13, color: scheme.onSurface),
          ),
          const SizedBox(height: 10),
          Row(
            children: <Widget>[
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: submitting ? null : onReject,
                  style: OutlinedButton.styleFrom(
                    minimumSize: const Size(0, 52),
                    foregroundColor: scheme.error,
                    side: BorderSide(color: scheme.errorContainer, width: 1.5),
                    textStyle: theme.textTheme.labelLarge?.copyWith(
                      fontSize: 14.5,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  icon: const Icon(Symbols.close_rounded, size: 20),
                  label: Text(l10n.approvalDetailReject),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                flex: 2,
                child: FilledButton.icon(
                  onPressed: submitting ? null : onApprove,
                  style: FilledButton.styleFrom(
                    minimumSize: const Size(0, 52),
                    textStyle: theme.textTheme.labelLarge?.copyWith(
                      fontSize: 14.5,
                      fontWeight: FontWeight.w700,
                      color: scheme.onPrimary,
                    ),
                  ),
                  icon: submitting
                      ? SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(
                            strokeWidth: 2.5,
                            color: scheme.onPrimary,
                          ),
                        )
                      : const Icon(Symbols.check_rounded, size: 20),
                  label: Text(l10n.approvalDetailApprove),
                ),
              ),
            ],
          ),
        ],
      );
    }

    return Container(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 12),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        border: Border(top: BorderSide(color: scheme.outlineVariant)),
      ),
      child: content,
    );
  }
}

/// Banner kaki (status akhir / SoD): pill lebar dengan ikon + teks.
class _FooterBanner extends StatelessWidget {
  const _FooterBanner({
    required this.label,
    required this.variant,
    required this.icon,
  });

  final String label;
  final StatusChipVariant variant;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    final StatusColorSet colors = statusColorSetOf(context, variant);

    return Container(
      constraints: const BoxConstraints(minHeight: 50),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        color: colors.bg,
        borderRadius: BorderRadius.circular(13),
      ),
      child: Row(
        children: <Widget>[
          Icon(icon, size: 20, color: colors.text),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              label,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w700,
                color: colors.text,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Dialog konfirmasi tolak (mockup): ikon report merah, judul, isi, kutipan
/// catatan (bila diisi), lalu Batal + "Ya, Tolak" destruktif. Varian lokal
/// dari [ConfirmDialog] karena membawa blok kutipan catatan.
class _RejectConfirmDialog extends StatelessWidget {
  const _RejectConfirmDialog({
    required this.title,
    required this.message,
    required this.confirmLabel,
    required this.noteLabel,
    this.note,
  });

  final String title;
  final String message;
  final String confirmLabel;
  final String noteLabel;
  final String? note;

  static Future<bool> show(
    BuildContext context, {
    required String title,
    required String message,
    required String confirmLabel,
    required String noteLabel,
    String? note,
  }) async {
    final bool? confirmed = await showDialog<bool>(
      context: context,
      builder: (BuildContext context) => _RejectConfirmDialog(
        title: title,
        message: message,
        confirmLabel: confirmLabel,
        noteLabel: noteLabel,
        note: note,
      ),
    );
    return confirmed ?? false;
  }

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String? noteText = note;

    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
      child: Padding(
        padding: const EdgeInsets.all(22),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: scheme.errorContainer,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Symbols.report_rounded,
                size: 26,
                color: scheme.error,
              ),
            ),
            const SizedBox(height: 14),
            Text(
              title,
              style: theme.textTheme.titleMedium?.copyWith(fontSize: 17),
            ),
            const SizedBox(height: 6),
            Text(
              message,
              style: TextStyle(
                fontSize: 13,
                height: 1.5,
                color: scheme.onSurfaceVariant,
              ),
            ),
            if (noteText != null) ...<Widget>[
              const SizedBox(height: 8),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 9,
                ),
                decoration: BoxDecoration(
                  color: scheme.secondaryContainer,
                  borderRadius: BorderRadius.circular(11),
                  border: Border.all(color: scheme.outlineVariant),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      noteLabel,
                      style: TextStyle(
                        fontSize: 10.5,
                        color: theme.textTheme.labelSmall?.color,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      '"$noteText"',
                      style: TextStyle(fontSize: 12.5, color: scheme.onSurface),
                    ),
                  ],
                ),
              ),
            ],
            const SizedBox(height: 18),
            Row(
              children: <Widget>[
                Expanded(
                  child: OutlinedButton(
                    style: OutlinedButton.styleFrom(
                      minimumSize: const Size(0, 46),
                      side: BorderSide(color: scheme.outline, width: 1.5),
                      foregroundColor: theme.textTheme.labelLarge?.color,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                      textStyle: theme.textTheme.labelLarge?.copyWith(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    onPressed: () => Navigator.of(context).pop(false),
                    child: Text(l10n.commonCancel),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: FilledButton(
                    style: FilledButton.styleFrom(
                      minimumSize: const Size(0, 46),
                      backgroundColor: scheme.error,
                      foregroundColor: scheme.onError,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                      textStyle: theme.textTheme.labelLarge?.copyWith(
                        fontSize: 14,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    onPressed: () => Navigator.of(context).pop(true),
                    child: Text(confirmLabel),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
