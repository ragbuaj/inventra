import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/api/app_failure.dart';
import '../../../core/connectivity/connectivity_provider.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/empty_state.dart';
import '../../../core/widgets/offline_banner.dart';
import '../../../core/widgets/status_chip.dart';
import '../../../core/widgets/sync_pill.dart';
import '../../../core/camera/scan_camera.dart';
import '../data/stock_opname_item_dto.dart';
import '../data/stock_opname_repository.dart';
import '../data/stock_opname_scan_result_dto.dart';
import '../data/stock_opname_session_dto.dart';
import 'opname_presentation.dart';
import 'opname_session_detail_provider.dart';

/// Layar Opname Counting 1:1 mockup "Inventra Mobile - Opname Counting":
/// header sesi, kartu progress (ring x/total + rekap hasil + SyncPill),
/// tombol "Pindai Aset Berikutnya" (kamera via [ScanCamera]) + "Ketik kode"
/// (input manual), daftar "Baru saja dipindai", dan bottom sheet hasil scan
/// (Ditemukan/Rusak/Salah Lokasi + catatan) yang menyimpan lewat
/// `PATCH .../items/{itemId}`.
///
/// Fase M0 online-only: saat offline pemindaian DINONAKTIFKAN dengan pesan
/// jelas (banner `opnameOfflineBanner`); TANPA antrean lokal — ikon sync per
/// baris selalu tersinkron karena setiap hasil langsung tercatat server.
class OpnameCountingScreen extends ConsumerStatefulWidget {
  const OpnameCountingScreen({required this.sessionId, super.key});

  final String sessionId;

  @override
  ConsumerState<OpnameCountingScreen> createState() =>
      _OpnameCountingScreenState();
}

class _OpnameCountingScreenState extends ConsumerState<OpnameCountingScreen> {
  bool _submitting = false;

  Future<void> _refresh() async {
    ref.invalidate(opnameSessionDetailProvider(widget.sessionId));
    try {
      await ref.read(opnameSessionDetailProvider(widget.sessionId).future);
    } on Object {
      // Kegagalan refresh sudah tercermin sebagai state error layar.
    }
  }

  Future<void> _openCameraScan() async {
    final String? tag = await Navigator.of(context).push<String>(
      MaterialPageRoute<String>(
        fullscreenDialog: true,
        builder: (BuildContext context) => const _CountingScanPage(),
      ),
    );
    if (tag != null && mounted) {
      await _submitTag(tag);
    }
  }

  Future<void> _openManualEntry() async {
    final ThemeData theme = Theme.of(context);
    final String? tag = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      backgroundColor: theme.cardTheme.color ?? theme.colorScheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      builder: (BuildContext sheetContext) => Padding(
        padding: EdgeInsets.only(
          bottom: MediaQuery.viewInsetsOf(sheetContext).bottom,
        ),
        child: const _ManualTagSheet(),
      ),
    );
    if (tag != null && mounted) {
      await _submitTag(tag);
    }
  }

  /// Resolve tag ke server, segarkan daftar item, lalu buka sheet hasil untuk
  /// item yang ter-resolve (match snapshot maupun temuan di luar catatan).
  Future<void> _submitTag(String rawTag) async {
    final String tag = rawTag.trim();
    if (tag.isEmpty || _submitting) {
      return;
    }
    _submitting = true;
    try {
      final StockOpnameScanResultDto scan = await ref
          .read(stockOpnameRepositoryProvider)
          .scan(widget.sessionId, tag);
      ref.invalidate(opnameSessionDetailProvider(widget.sessionId));
      final OpnameSessionDetail detail = await ref.read(
        opnameSessionDetailProvider(widget.sessionId).future,
      );
      if (!mounted) {
        return;
      }
      StockOpnameItemDto? item;
      for (final StockOpnameItemDto candidate in detail.items) {
        if (candidate.id == scan.id) {
          item = candidate;
          break;
        }
      }
      if (item != null) {
        await _openResultSheet(item);
      }
    } on AppFailure catch (failure) {
      _showFailureSnack(failure, tag: tag);
    } finally {
      _submitting = false;
    }
  }

  Future<void> _openResultSheet(StockOpnameItemDto item) async {
    final ThemeData theme = Theme.of(context);
    final bool? saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      backgroundColor: theme.cardTheme.color ?? theme.colorScheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      builder: (BuildContext sheetContext) => Padding(
        padding: EdgeInsets.only(
          bottom: MediaQuery.viewInsetsOf(sheetContext).bottom,
        ),
        child: _ResultSheet(sessionId: widget.sessionId, item: item),
      ),
    );
    if (saved ?? false) {
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(AppLocalizations.of(context).opnameResultSavedSnack),
        ),
      );
      await _refresh();
    }
  }

  void _showFailureSnack(AppFailure failure, {required String tag}) {
    if (!mounted) {
      return;
    }
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String message = switch (failure) {
      NotFoundFailure() => l10n.opnameScanErrorNotFound(tag),
      ConflictFailure() => l10n.opnameScanErrorNotCounting,
      NetworkFailure() => l10n.opnameErrorNetworkBody,
      _ => l10n.opnameErrorGenericBody,
    };
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final AsyncValue<OpnameSessionDetail> state = ref.watch(
      opnameSessionDetailProvider(widget.sessionId),
    );
    final bool offline = isOffline(ref.watch(isOnlineProvider));
    final StockOpnameSessionDto? session = state.value?.session;

    return Scaffold(
      appBar: AppBar(
        title: session == null
            ? Text(l10n.opnameDetailTitle)
            : Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(
                    opnameSessionTitle(session),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  Text(
                    opnameSessionSubtitle(session, localeName),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w400,
                      color: Theme.of(context).textTheme.bodySmall?.color,
                    ),
                  ),
                ],
              ),
        actions: <Widget>[
          IconButton(
            tooltip: l10n.opnameCountingVarianceTooltip,
            onPressed: () =>
                context.push('/stock-opname/${widget.sessionId}/variance'),
            icon: const Icon(Symbols.rule_rounded),
          ),
        ],
      ),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            if (offline)
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 4),
                child: OfflineBanner(message: l10n.opnameOfflineBanner),
              ),
            Expanded(
              child: state.when(
                data: (OpnameSessionDetail detail) => _CountingBody(
                  detail: detail,
                  offline: offline,
                  onScan: _openCameraScan,
                  onManual: _openManualEntry,
                  onRefresh: _refresh,
                  onItemTap: _openResultSheet,
                ),
                loading: () => const _LoadingSkeleton(),
                error: (Object error, StackTrace stackTrace) => _ErrorState(
                  failure: error,
                  onRetry: () => ref.invalidate(
                    opnameSessionDetailProvider(widget.sessionId),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Isi layar saat data termuat: kartu progress, tombol scan, daftar terbaru.
class _CountingBody extends StatelessWidget {
  const _CountingBody({
    required this.detail,
    required this.offline,
    required this.onScan,
    required this.onManual,
    required this.onRefresh,
    required this.onItemTap,
  });

  final OpnameSessionDetail detail;
  final bool offline;
  final VoidCallback onScan;
  final VoidCallback onManual;
  final Future<void> Function() onRefresh;
  final ValueChanged<StockOpnameItemDto> onItemTap;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);
    final List<StockOpnameItemDto> counted = detail.counted;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 2, 20, 0),
          child: _ProgressCard(detail: detail, offline: offline),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 4),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              FilledButton.icon(
                style: FilledButton.styleFrom(
                  minimumSize: const Size.fromHeight(58),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                  ),
                  textStyle: theme.textTheme.labelLarge?.copyWith(
                    fontSize: 15.5,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                onPressed: offline ? null : onScan,
                icon: const Icon(Symbols.qr_code_scanner_rounded, size: 24),
                label: Text(l10n.opnameCountingScanButton),
              ),
              const SizedBox(height: 4),
              TextButton.icon(
                style: TextButton.styleFrom(
                  minimumSize: const Size.fromHeight(44),
                  textStyle: theme.textTheme.labelLarge?.copyWith(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                onPressed: offline ? null : onManual,
                icon: const Icon(Symbols.keyboard_rounded, size: 17),
                label: Text(l10n.opnameCountingManualButton),
              ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 4, 20, 8),
          child: Text(
            l10n.opnameCountingRecentHeader.toUpperCase(),
            style: TextStyle(
              fontSize: 11.5,
              fontWeight: FontWeight.w700,
              letterSpacing: 0.7,
              color: theme.textTheme.labelSmall?.color,
            ),
          ),
        ),
        Expanded(
          child: counted.isEmpty
              ? Center(
                  child: Text(
                    l10n.opnameCountingRecentEmpty,
                    style: TextStyle(
                      fontSize: 12,
                      color: theme.textTheme.labelSmall?.color,
                    ),
                  ),
                )
              : RefreshIndicator(
                  onRefresh: onRefresh,
                  child: ListView.separated(
                    physics: const AlwaysScrollableScrollPhysics(),
                    padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
                    itemCount: counted.length,
                    separatorBuilder: (BuildContext context, int index) =>
                        const SizedBox(height: 8),
                    itemBuilder: (BuildContext context, int index) =>
                        _CountedItemRow(
                          item: counted[index],
                          onTap: () => onItemTap(counted[index]),
                        ),
                  ),
                ),
        ),
      ],
    );
  }
}

/// Kartu progress sticky: ring counted/total + rekap hasil + SyncPill.
class _ProgressCard extends StatelessWidget {
  const _ProgressCard({required this.detail, required this.offline});

  final OpnameSessionDetail detail;
  final bool offline;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final InventraStatusColors colors = theme
        .extension<InventraStatusColors>()!;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final StockOpnameSessionDto session = detail.session;
    // KPI detail sesi bila ada; fallback hitung dari daftar item.
    final int total = session.total ?? detail.items.length;
    final int counted =
        opnameCountedOf(session) ??
        detail.items
            .where((StockOpnameItemDto item) => item.result != 'pending')
            .length;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        children: <Widget>[
          SizedBox(
            width: 72,
            height: 72,
            child: Stack(
              fit: StackFit.expand,
              children: <Widget>[
                CircularProgressIndicator(
                  value: total == 0 ? 0 : counted / total,
                  strokeWidth: 9,
                  strokeCap: StrokeCap.round,
                  backgroundColor: scheme.outlineVariant,
                ),
                Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      Text(
                        '$counted',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w800,
                          height: 1.1,
                          color: scheme.onPrimaryContainer,
                        ),
                      ),
                      Text(
                        l10n.opnameCountingRingTotal(total),
                        style: TextStyle(
                          fontSize: 9.5,
                          color: theme.textTheme.labelSmall?.color,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Wrap(
                  spacing: 10,
                  runSpacing: 4,
                  children: <Widget>[
                    _StatChip(
                      icon: Symbols.check_circle_rounded,
                      count: detail.countOf('found'),
                      color: colors.success.text,
                    ),
                    _StatChip(
                      icon: Symbols.build_rounded,
                      count: detail.countOf('damaged'),
                      color: colors.warning.text,
                    ),
                    _StatChip(
                      icon: Symbols.wrong_location_rounded,
                      count: detail.countOf('misplaced'),
                      color: colors.info.text,
                    ),
                  ],
                ),
                const SizedBox(height: 6),
                SyncPill(
                  status: offline
                      ? SyncPillStatus.offline
                      : SyncPillStatus.synced,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Rekap kecil hasil per kategori pada kartu progress (ikon + angka).
class _StatChip extends StatelessWidget {
  const _StatChip({
    required this.icon,
    required this.count,
    required this.color,
  });

  final IconData icon;
  final int count;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        Icon(icon, size: 14, color: color),
        const SizedBox(width: 4),
        Text(
          '$count',
          style: TextStyle(
            fontSize: 11.5,
            fontWeight: FontWeight.w600,
            color: color,
          ),
        ),
      ],
    );
  }
}

/// Baris "Baru saja dipindai": nama aset, tag · jam (+ penanda di luar
/// catatan), chip hasil, dan ikon tersinkron (online-only — hasil langsung
/// tercatat server).
class _CountedItemRow extends StatelessWidget {
  const _CountedItemRow({required this.item, required this.onTap});

  final StockOpnameItemDto item;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final InventraStatusColors colors = theme
        .extension<InventraStatusColors>()!;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final (String resultLabel, StatusChipVariant resultVariant) =
        opnameItemResultPresentation(l10n, item.result);
    final DateTime? countedAt = item.countedAt;
    final String subtitle = <String>[
      if (item.assetTag != null) item.assetTag!,
      if (countedAt != null) opnameCountedTime(countedAt, localeName),
      if (!item.expected) l10n.opnameOutOfSnapshot,
    ].join(' · ');

    return Material(
      color: theme.cardTheme.color ?? scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(color: scheme.outlineVariant),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 11),
          child: Row(
            children: <Widget>[
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      item.assetName ?? item.assetId,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: scheme.onSurface,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      subtitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 10.5,
                        color: theme.textTheme.labelSmall?.color,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 10),
              StatusChip(label: resultLabel, variant: resultVariant),
              const SizedBox(width: 8),
              Icon(
                Symbols.check_circle_rounded,
                size: 16,
                color: colors.success.dot,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Halaman kamera full screen untuk scan bar counting — reuse abstraksi
/// [ScanCamera] (stub deterministik di test/golden). Deteksi pertama menutup
/// halaman dengan nilai tag; tombol tutup mengembalikan null.
class _CountingScanPage extends ConsumerStatefulWidget {
  const _CountingScanPage();

  @override
  ConsumerState<_CountingScanPage> createState() => _CountingScanPageState();
}

class _CountingScanPageState extends ConsumerState<_CountingScanPage> {
  late final ScanCamera _camera;
  bool _popped = false;

  @override
  void initState() {
    super.initState();
    _camera = ref.read(scanCameraFactoryProvider)();
  }

  @override
  void dispose() {
    _camera.dispose();
    super.dispose();
  }

  /// Kamera mengirim deteksi berulang untuk label yang sama — hanya deteksi
  /// pertama yang menutup halaman.
  void _onDetect(String tag) {
    if (_popped || tag.trim().isEmpty) {
      return;
    }
    _popped = true;
    Navigator.of(context).pop(tag.trim());
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle.light,
      child: Scaffold(
        backgroundColor: InventraScanColors.viewfinderBackground,
        body: ValueListenableBuilder<bool>(
          valueListenable: _camera.unavailable,
          builder: (BuildContext context, bool unavailable, Widget? child) {
            return Stack(
              fit: StackFit.expand,
              children: <Widget>[
                if (!unavailable) _camera.buildPreview(onDetect: _onDetect),
                SafeArea(
                  child: Column(
                    children: <Widget>[
                      Padding(
                        padding: const EdgeInsets.fromLTRB(20, 14, 20, 0),
                        child: Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: <Widget>[
                            _ScanOverlayButton(
                              icon: Symbols.close_rounded,
                              tooltip: l10n.scanCloseTooltip,
                              onPressed: () => Navigator.of(context).pop(),
                            ),
                            Text(
                              l10n.scanTitle,
                              style: const TextStyle(
                                fontSize: 15,
                                fontWeight: FontWeight.w700,
                                color: InventraScanColors.foreground,
                              ),
                            ),
                            ValueListenableBuilder<bool>(
                              valueListenable: _camera.torchOn,
                              builder:
                                  (
                                    BuildContext context,
                                    bool torchOn,
                                    Widget? child,
                                  ) => _ScanOverlayButton(
                                    icon: torchOn
                                        ? Symbols.flashlight_off_rounded
                                        : Symbols.flashlight_on_rounded,
                                    tooltip: torchOn
                                        ? l10n.scanTorchOffTooltip
                                        : l10n.scanTorchOnTooltip,
                                    enabled: !unavailable,
                                    onPressed: () => _camera.toggleTorch(),
                                  ),
                            ),
                          ],
                        ),
                      ),
                      Expanded(
                        child: Center(
                          child: unavailable
                              ? _ScanUnavailable(l10n: l10n)
                              : Container(
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 16,
                                    vertical: 8,
                                  ),
                                  decoration: const ShapeDecoration(
                                    color: InventraScanColors.controlBackground,
                                    shape: StadiumBorder(),
                                  ),
                                  child: Text(
                                    l10n.scanHint,
                                    style: const TextStyle(
                                      fontSize: 13,
                                      color: InventraScanColors.foregroundMuted,
                                    ),
                                  ),
                                ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

/// Tombol lingkaran semi-transparan overlay kamera (tutup/torch).
class _ScanOverlayButton extends StatelessWidget {
  const _ScanOverlayButton({
    required this.icon,
    required this.tooltip,
    required this.onPressed,
    this.enabled = true,
  });

  final IconData icon;
  final String tooltip;
  final VoidCallback onPressed;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: InventraScanColors.controlBackground,
        shape: const CircleBorder(),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: enabled ? onPressed : null,
          child: SizedBox(
            width: 44,
            height: 44,
            child: Icon(
              icon,
              size: 22,
              color: enabled
                  ? InventraScanColors.foreground
                  : InventraScanColors.foregroundMuted,
            ),
          ),
        ),
      ),
    );
  }
}

/// State kamera tidak tersedia pada halaman scan counting.
class _ScanUnavailable extends StatelessWidget {
  const _ScanUnavailable({required this.l10n});

  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 40),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          Container(
            width: 56,
            height: 56,
            decoration: const BoxDecoration(
              color: InventraScanColors.controlBackground,
              shape: BoxShape.circle,
            ),
            child: const Icon(
              Symbols.no_photography_rounded,
              size: 30,
              color: InventraScanColors.foregroundMuted,
            ),
          ),
          const SizedBox(height: 12),
          Text(
            l10n.scanCameraUnavailableTitle,
            textAlign: TextAlign.center,
            style: const TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: InventraScanColors.foreground,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            l10n.scanCameraUnavailableBody,
            textAlign: TextAlign.center,
            style: const TextStyle(
              fontSize: 12,
              color: InventraScanColors.foregroundMuted,
            ),
          ),
        ],
      ),
    );
  }
}

/// Bottom sheet input tag manual (jalur "Ketik kode") — memakai kunci i18n
/// scanManual* yang sama dengan layar Scan.
class _ManualTagSheet extends StatefulWidget {
  const _ManualTagSheet();

  @override
  State<_ManualTagSheet> createState() => _ManualTagSheetState();
}

class _ManualTagSheetState extends State<_ManualTagSheet> {
  final TextEditingController _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _submit() {
    final String tag = _controller.text.trim();
    if (tag.isEmpty) {
      return;
    }
    Navigator.of(context).pop(tag);
  }

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 18),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            l10n.scanManualSheetTitle,
            style: theme.textTheme.titleMedium?.copyWith(fontSize: 16),
          ),
          const SizedBox(height: 14),
          TextField(
            controller: _controller,
            autofocus: true,
            textCapitalization: TextCapitalization.characters,
            textInputAction: TextInputAction.search,
            onSubmitted: (_) => _submit(),
            style: const TextStyle(fontSize: 15),
            decoration: InputDecoration(
              hintText: l10n.scanManualFieldHint,
              prefixIcon: Padding(
                padding: const EdgeInsets.only(left: 16, right: 10),
                child: Icon(
                  Symbols.tag_rounded,
                  size: 20,
                  color: scheme.primary,
                ),
              ),
              prefixIconConstraints: const BoxConstraints(
                minWidth: 0,
                minHeight: 0,
              ),
            ),
          ),
          const SizedBox(height: 7),
          Text(
            l10n.scanManualFieldHelper,
            style: TextStyle(
              fontSize: 11,
              color: theme.textTheme.labelSmall?.color,
            ),
          ),
          const SizedBox(height: 14),
          FilledButton.icon(
            style: FilledButton.styleFrom(
              minimumSize: const Size.fromHeight(
                InventraDimens.buttonHeightPrimary,
              ),
              textStyle: theme.textTheme.labelLarge?.copyWith(
                fontSize: 15,
                fontWeight: FontWeight.w700,
              ),
            ),
            onPressed: _submit,
            icon: const Icon(Symbols.search_rounded, size: 20),
            label: Text(l10n.scanManualSubmit),
          ),
        ],
      ),
    );
  }
}

/// Bottom sheet hasil scan 1:1 mockup: header aset, pemilih hasil
/// (Ditemukan/Rusak/Salah Lokasi), catatan opsional, "Simpan & Lanjut" —
/// menyimpan via `PATCH .../items/{itemId}` lalu pop(true).
class _ResultSheet extends ConsumerStatefulWidget {
  const _ResultSheet({required this.sessionId, required this.item});

  final String sessionId;
  final StockOpnameItemDto item;

  @override
  ConsumerState<_ResultSheet> createState() => _ResultSheetState();
}

class _ResultSheetState extends ConsumerState<_ResultSheet> {
  late final TextEditingController _noteController = TextEditingController(
    text: widget.item.note,
  );
  late OpnameItemResult _selected = switch (OpnameItemResult.tryParse(
    widget.item.result,
  )) {
    OpnameItemResult.damaged => OpnameItemResult.damaged,
    OpnameItemResult.misplaced => OpnameItemResult.misplaced,
    _ => OpnameItemResult.found,
  };
  bool _saving = false;

  @override
  void dispose() {
    _noteController.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (_saving) {
      return;
    }
    setState(() => _saving = true);
    try {
      await ref
          .read(stockOpnameRepositoryProvider)
          .setResult(
            widget.sessionId,
            widget.item.id,
            result: _selected,
            note: _noteController.text,
          );
      if (mounted) {
        Navigator.of(context).pop(true);
      }
    } on AppFailure catch (failure) {
      if (!mounted) {
        return;
      }
      setState(() => _saving = false);
      final AppLocalizations l10n = AppLocalizations.of(context);
      final String message = switch (failure) {
        ConflictFailure() => l10n.opnameScanErrorNotCounting,
        NetworkFailure() => l10n.opnameErrorNetworkBody,
        _ => l10n.opnameErrorGenericBody,
      };
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(message)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final InventraStatusColors colors = theme
        .extension<InventraStatusColors>()!;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final StockOpnameItemDto item = widget.item;
    final String subtitle = <String>[
      if (item.assetTag != null) item.assetTag!,
      if (item.roomName != null) item.roomName!,
    ].join(' · ');

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 14),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Container(
                width: 50,
                height: 50,
                decoration: BoxDecoration(
                  color: scheme.secondaryContainer,
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Icon(
                  Symbols.inventory_2_rounded,
                  size: 24,
                  color: theme.textTheme.labelSmall?.color,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      item.assetName ?? item.assetId,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    if (subtitle.isNotEmpty)
                      Text(
                        subtitle,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 11,
                          color: theme.textTheme.bodySmall?.color,
                        ),
                      ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Icon(
                Symbols.check_circle_rounded,
                size: 22,
                color: colors.success.dot,
              ),
            ],
          ),
          if (!item.expected) ...<Widget>[
            const SizedBox(height: 12),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
              decoration: BoxDecoration(
                color: colors.neutral.bg,
                borderRadius: BorderRadius.circular(11),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Icon(
                    Symbols.info_rounded,
                    size: 15,
                    color: colors.neutral.text,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      l10n.opnameSheetOutOfSnapshotInfo,
                      style: TextStyle(
                        fontSize: 11.5,
                        height: 1.45,
                        color: colors.neutral.text,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
          const SizedBox(height: 14),
          Text(
            l10n.opnameSheetResultLabel,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w700,
              color: theme.textTheme.bodySmall?.color,
            ),
          ),
          const SizedBox(height: 8),
          _ResultPicker(
            selected: _selected,
            onSelect: (OpnameItemResult result) =>
                setState(() => _selected = result),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _noteController,
            textInputAction: TextInputAction.done,
            style: const TextStyle(fontSize: 13),
            decoration: InputDecoration(hintText: l10n.opnameSheetNoteHint),
          ),
          const SizedBox(height: 12),
          FilledButton.icon(
            style: FilledButton.styleFrom(
              minimumSize: const Size.fromHeight(54),
              textStyle: theme.textTheme.labelLarge?.copyWith(
                fontSize: 15,
                fontWeight: FontWeight.w700,
              ),
            ),
            onPressed: _saving ? null : _save,
            icon: _saving
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2.5),
                  )
                : const Icon(Symbols.save_rounded, size: 21),
            label: Text(l10n.opnameSheetSave),
          ),
        ],
      ),
    );
  }
}

/// Pemilih hasil tiga segmen (Ditemukan/Rusak/Salah Lokasi) 1:1 mockup;
/// segmen terpilih terisi warna keluarga semantiknya.
class _ResultPicker extends StatelessWidget {
  const _ResultPicker({required this.selected, required this.onSelect});

  final OpnameItemResult selected;
  final ValueChanged<OpnameItemResult> onSelect;

  static const List<OpnameItemResult> _options = <OpnameItemResult>[
    OpnameItemResult.found,
    OpnameItemResult.damaged,
    OpnameItemResult.misplaced,
  ];

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final InventraStatusColors colors = theme
        .extension<InventraStatusColors>()!;
    final AppLocalizations l10n = AppLocalizations.of(context);

    (String, Color) presentation(OpnameItemResult result) => switch (result) {
      OpnameItemResult.found => (l10n.opnameResultFound, colors.success.dot),
      OpnameItemResult.damaged => (
        l10n.opnameResultDamaged,
        colors.warning.dot,
      ),
      _ => (l10n.opnameResultMisplaced, colors.info.dot),
    };

    return Container(
      decoration: BoxDecoration(
        border: Border.all(color: scheme.outline),
        borderRadius: BorderRadius.circular(14),
      ),
      clipBehavior: Clip.antiAlias,
      child: Row(
        children: <Widget>[
          for (final OpnameItemResult option in _options)
            Expanded(
              child: Builder(
                builder: (BuildContext context) {
                  final (String label, Color fill) = presentation(option);
                  final bool active = option == selected;

                  return Semantics(
                    button: true,
                    selected: active,
                    child: Material(
                      color: active
                          ? fill
                          : theme.cardTheme.color ?? scheme.surface,
                      child: InkWell(
                        onTap: () => onSelect(option),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 4,
                            vertical: 11,
                          ),
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: <Widget>[
                              Icon(
                                opnameItemResultIcon(option.wire),
                                size: 20,
                                color: active
                                    ? scheme.surface
                                    : theme.textTheme.bodySmall?.color,
                              ),
                              const SizedBox(height: 3),
                              Text(
                                label,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(
                                  fontSize: 11,
                                  fontWeight: active
                                      ? FontWeight.w700
                                      : FontWeight.w600,
                                  color: active
                                      ? scheme.surface
                                      : theme.textTheme.bodySmall?.color,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  );
                },
              ),
            ),
        ],
      ),
    );
  }
}

/// Skeleton loading counting: kerangka kartu progress + tombol + baris item.
class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 2, 20, 20),
      children: const <Widget>[
        AppSkeleton(height: 100, borderRadius: 18),
        SizedBox(height: 12),
        AppSkeleton(height: 58, borderRadius: 16),
        SizedBox(height: 8),
        AppSkeleton(height: 44, borderRadius: 12),
        SizedBox(height: 16),
        AppSkeleton(height: 11, width: 140, borderRadius: 6),
        SizedBox(height: 10),
        AppSkeleton(height: 56, borderRadius: 14),
        SizedBox(height: 8),
        AppSkeleton(height: 56, borderRadius: 14),
        SizedBox(height: 8),
        AppSkeleton(height: 56, borderRadius: 14),
      ],
    );
  }
}

/// Cabang error detail sesi: offline, 404, 403, dan generik.
class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.failure, required this.onRetry});

  final Object failure;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return switch (failure) {
      NetworkFailure() => EmptyState(
        icon: Symbols.wifi_off_rounded,
        title: l10n.opnameDetailErrorTitle,
        subtitle: l10n.opnameErrorNetworkBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      NotFoundFailure() => EmptyState(
        icon: Symbols.search_off_rounded,
        title: l10n.opnameDetailNotFoundTitle,
        subtitle: l10n.opnameDetailNotFoundBody,
      ),
      ForbiddenFailure() => EmptyState(
        icon: Symbols.lock_rounded,
        title: l10n.opnameForbiddenTitle,
        subtitle: l10n.opnameForbiddenBody,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.opnameDetailErrorTitle,
        subtitle: l10n.opnameErrorGenericBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}
