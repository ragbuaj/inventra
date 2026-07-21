import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../../core/api/failure_message.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../catalog/data/filter_options_repository.dart';
import '../data/asset_register_repository.dart';

/// Registrasi Aset (1:1 mockup "Inventra Mobile - Form Registrasi Aset"): form
/// stepper 3 langkah -> `POST /requests` type `asset_create`. Field mengikuti
/// AssetCreatePayload; harga perolehan numerik-only (amount == purchase_cost).
/// Tanpa cek ambang kapitalisasi (tak ada di web; server selalu mengkapitalisasi).
///
/// Field referensi opsional (brand/model/unit/vendor/ruangan) menyusul.
class AssetRegisterScreen extends ConsumerStatefulWidget {
  const AssetRegisterScreen({super.key});

  @override
  ConsumerState<AssetRegisterScreen> createState() =>
      _AssetRegisterScreenState();
}

class _AssetRegisterScreenState extends ConsumerState<AssetRegisterScreen> {
  final TextEditingController _name = TextEditingController();
  final TextEditingController _serial = TextEditingController();
  final TextEditingController _cost = TextEditingController();
  final TextEditingController _notes = TextEditingController();

  int _step = 0;
  FilterOption? _category;
  FilterOption? _office;
  String _assetClass = 'tangible';
  DateTime? _purchaseDate;
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _name.dispose();
    _serial.dispose();
    _cost.dispose();
    _notes.dispose();
    super.dispose();
  }

  bool _validateStep(int step) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    if (step == 0) {
      if (_name.text.trim().isEmpty) {
        setState(() => _error = l10n.registerNameRequired);
        return false;
      }
      if (_category == null) {
        setState(() => _error = l10n.registerCategoryRequired);
        return false;
      }
    }
    if (step == 1) {
      if (_office == null) {
        setState(() => _error = l10n.registerOfficeRequired);
        return false;
      }
    }
    setState(() => _error = null);
    return true;
  }

  void _next() {
    if (!_validateStep(_step)) {
      return;
    }
    if (_step < 2) {
      setState(() => _step += 1);
    }
  }

  void _back() {
    if (_step > 0) {
      setState(() {
        _step -= 1;
        _error = null;
      });
    }
  }

  Future<void> _submit() async {
    if (!_validateStep(0) || !_validateStep(1)) {
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    final AppLocalizations l10n = AppLocalizations.of(context);
    try {
      await ref
          .read(assetRegisterRepositoryProvider)
          .register(
            name: _name.text,
            categoryId: _category!.id,
            officeId: _office!.id,
            assetClass: _assetClass,
            purchaseCost: _cost.text,
            purchaseDate: _purchaseDate == null
                ? null
                : DateFormat('yyyy-MM-dd').format(_purchaseDate!),
            serialNumber: _serial.text,
            notes: _notes.text,
          );
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.registerSuccess)));
      // Arahkan ke Pengajuan Saya (lensa maker).
      context.go('/my-requests');
    } on Object catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = actionFailureMessage(e, l10n, fallback: l10n.registerError);
      });
    }
  }

  Future<void> _pickDate() async {
    final DateTime now = DateTime.now();
    final DateTime? picked = await showDatePicker(
      context: context,
      firstDate: DateTime(now.year - 30),
      lastDate: now,
      initialDate: _purchaseDate ?? now,
    );
    if (picked != null) {
      setState(() => _purchaseDate = picked);
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Scaffold(
      appBar: AppBar(title: Text(l10n.registerTitle)),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            Expanded(
              child: Stepper(
                type: StepperType.horizontal,
                currentStep: _step,
                onStepContinue: _next,
                onStepCancel: _back,
                onStepTapped: (int s) {
                  // Hanya boleh mundur bebas; maju lewat validasi.
                  if (s < _step) {
                    setState(() {
                      _step = s;
                      _error = null;
                    });
                  }
                },
                controlsBuilder: (BuildContext context, ControlsDetails d) =>
                    const SizedBox.shrink(),
                steps: <Step>[
                  Step(
                    title: Text(l10n.registerStepIdentity),
                    isActive: _step >= 0,
                    state: _step > 0 ? StepState.complete : StepState.indexed,
                    content: _IdentityStep(
                      name: _name,
                      serial: _serial,
                      category: _category,
                      onCategory: (FilterOption? c) =>
                          setState(() => _category = c),
                      assetClass: _assetClass,
                      onAssetClass: (String c) =>
                          setState(() => _assetClass = c),
                    ),
                  ),
                  Step(
                    title: Text(l10n.registerStepPlacement),
                    isActive: _step >= 1,
                    state: _step > 1 ? StepState.complete : StepState.indexed,
                    content: _PlacementStep(
                      office: _office,
                      onOffice: (FilterOption? o) => setState(() => _office = o),
                      cost: _cost,
                      notes: _notes,
                      purchaseDate: _purchaseDate,
                      onPickDate: _pickDate,
                    ),
                  ),
                  Step(
                    title: Text(l10n.registerStepReview),
                    isActive: _step >= 2,
                    state: StepState.indexed,
                    content: _ReviewStep(
                      name: _name.text,
                      category: _category?.name,
                      assetClass: _assetClass,
                      office: _office?.name,
                      cost: _cost.text,
                      serial: _serial.text,
                    ),
                  ),
                ],
              ),
            ),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 8),
                child: Text(
                  _error!,
                  style: TextStyle(
                    color: Theme.of(context).colorScheme.error,
                    fontSize: 13,
                  ),
                ),
              ),
            _ControlBar(
              step: _step,
              submitting: _submitting,
              onBack: _step == 0 ? null : _back,
              onNext: _step < 2 ? _next : null,
              onSubmit: _step == 2 ? _submit : null,
            ),
          ],
        ),
      ),
    );
  }
}

/// Bar kontrol bawah: Kembali + Lanjut/Kirim.
class _ControlBar extends StatelessWidget {
  const _ControlBar({
    required this.step,
    required this.submitting,
    required this.onBack,
    required this.onNext,
    required this.onSubmit,
  });

  final int step;
  final bool submitting;
  final VoidCallback? onBack;
  final VoidCallback? onNext;
  final VoidCallback? onSubmit;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    return Material(
      color: Theme.of(context).cardTheme.color,
      elevation: 8,
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 10, 20, 10),
          child: Row(
            children: <Widget>[
              if (onBack != null)
                Expanded(
                  child: OutlinedButton(
                    onPressed: submitting ? null : onBack,
                    style: OutlinedButton.styleFrom(
                      minimumSize: const Size(0, 48),
                    ),
                    child: Text(l10n.registerBack),
                  ),
                ),
              if (onBack != null) const SizedBox(width: 10),
              Expanded(
                child: FilledButton(
                  onPressed: submitting ? null : (onSubmit ?? onNext),
                  style: FilledButton.styleFrom(
                    minimumSize: const Size(0, 48),
                  ),
                  child: submitting
                      ? const SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(strokeWidth: 2.5),
                        )
                      : Text(
                          onSubmit != null
                              ? l10n.registerSubmit
                              : l10n.registerNext,
                        ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _IdentityStep extends StatelessWidget {
  const _IdentityStep({
    required this.name,
    required this.serial,
    required this.category,
    required this.onCategory,
    required this.assetClass,
    required this.onAssetClass,
  });

  final TextEditingController name;
  final TextEditingController serial;
  final FilterOption? category;
  final ValueChanged<FilterOption?> onCategory;
  final String assetClass;
  final ValueChanged<String> onAssetClass;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        TextField(
          controller: name,
          decoration: InputDecoration(labelText: l10n.registerName),
        ),
        const SizedBox(height: 12),
        _OptionDropdown(
          label: l10n.registerCategory,
          provider: catalogCategoryOptionsProvider,
          selected: category,
          onChanged: onCategory,
        ),
        const SizedBox(height: 12),
        Text(l10n.registerAssetClass),
        const SizedBox(height: 6),
        SegmentedButton<String>(
          segments: <ButtonSegment<String>>[
            ButtonSegment<String>(
              value: 'tangible',
              label: Text(l10n.registerClassTangible),
            ),
            ButtonSegment<String>(
              value: 'intangible',
              label: Text(l10n.registerClassIntangible),
            ),
          ],
          selected: <String>{assetClass},
          onSelectionChanged: (Set<String> s) => onAssetClass(s.first),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: serial,
          decoration: InputDecoration(labelText: l10n.registerSerial),
        ),
      ],
    );
  }
}

class _PlacementStep extends StatelessWidget {
  const _PlacementStep({
    required this.office,
    required this.onOffice,
    required this.cost,
    required this.notes,
    required this.purchaseDate,
    required this.onPickDate,
  });

  final FilterOption? office;
  final ValueChanged<FilterOption?> onOffice;
  final TextEditingController cost;
  final TextEditingController notes;
  final DateTime? purchaseDate;
  final VoidCallback onPickDate;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        _OptionDropdown(
          label: l10n.registerOffice,
          provider: catalogOfficeOptionsProvider,
          selected: office,
          onChanged: onOffice,
        ),
        const SizedBox(height: 12),
        TextField(
          controller: cost,
          keyboardType: TextInputType.number,
          // Digit-only (rupiah bulat, selaras decimalDigits:0 di ringkasan):
          // menolak keystroke non-angka termasuk titik ribuan yang membuat
          // purchase_cost/amount malformed lalu ditolak backend.
          inputFormatters: <TextInputFormatter>[
            FilteringTextInputFormatter.digitsOnly,
          ],
          decoration: InputDecoration(
            labelText: l10n.registerPurchaseCost,
            prefixText: 'Rp ',
          ),
        ),
        const SizedBox(height: 12),
        Text(l10n.registerPurchaseDate),
        const SizedBox(height: 6),
        OutlinedButton.icon(
          onPressed: onPickDate,
          icon: const Icon(Icons.calendar_month, size: 18),
          label: Text(
            purchaseDate == null
                ? l10n.borrowPickDate
                : DateFormat('d MMM y', localeName).format(purchaseDate!),
          ),
          style: OutlinedButton.styleFrom(
            alignment: Alignment.centerLeft,
            minimumSize: const Size.fromHeight(48),
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: notes,
          minLines: 2,
          maxLines: 4,
          decoration: InputDecoration(labelText: l10n.registerNotes),
        ),
      ],
    );
  }
}

class _ReviewStep extends StatelessWidget {
  const _ReviewStep({
    required this.name,
    required this.category,
    required this.assetClass,
    required this.office,
    required this.cost,
    required this.serial,
  });

  final String name;
  final String? category;
  final String assetClass;
  final String? office;
  final String cost;
  final String serial;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final String classLabel = assetClass == 'intangible'
        ? l10n.registerClassIntangible
        : l10n.registerClassTangible;
    final String costLabel = cost.trim().isEmpty
        ? '—'
        : NumberFormat.currency(
            locale: localeName,
            symbol: 'Rp ',
            decimalDigits: 0,
          ).format(double.tryParse(cost.trim()) ?? 0);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        _ReviewRow(label: l10n.registerName, value: name),
        _ReviewRow(label: l10n.registerCategory, value: category ?? '—'),
        _ReviewRow(label: l10n.registerAssetClass, value: classLabel),
        _ReviewRow(label: l10n.registerOffice, value: office ?? '—'),
        _ReviewRow(label: l10n.registerPurchaseCost, value: costLabel),
        if (serial.trim().isNotEmpty)
          _ReviewRow(label: l10n.registerSerial, value: serial.trim()),
        const SizedBox(height: 12),
        Text(
          l10n.registerReviewNote,
          style: TextStyle(
            fontSize: 12,
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

class _ReviewRow extends StatelessWidget {
  const _ReviewRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          SizedBox(
            width: 120,
            child: Text(
              label,
              style: TextStyle(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
                fontSize: 13,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value.trim().isEmpty ? '—' : value.trim(),
              style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
    );
  }
}

/// Dropdown opsi dari FutureProvider (kategori/kantor): loading, "Tidak ada
/// data", atau daftar. Menyimpan pilihan sebagai [FilterOption].
class _OptionDropdown extends ConsumerWidget {
  const _OptionDropdown({
    required this.label,
    required this.provider,
    required this.selected,
    required this.onChanged,
  });

  final String label;
  final FutureProvider<List<FilterOption>> provider;
  final FilterOption? selected;
  final ValueChanged<FilterOption?> onChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<List<FilterOption>> options = ref.watch(provider);

    return options.when(
      loading: () => InputDecorator(
        decoration: InputDecoration(labelText: label),
        child: const SizedBox(
          height: 20,
          child: Align(
            alignment: Alignment.centerLeft,
            child: SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(strokeWidth: 2.5),
            ),
          ),
        ),
      ),
      error: (Object e, StackTrace s) => InputDecorator(
        decoration: InputDecoration(labelText: label),
        child: Text(l10n.catalogFilterNoOptions),
      ),
      data: (List<FilterOption> list) {
        if (list.isEmpty) {
          return InputDecorator(
            decoration: InputDecoration(labelText: label),
            child: Text(l10n.catalogFilterNoOptions),
          );
        }
        return DropdownButtonFormField<String>(
          initialValue: selected?.id,
          isExpanded: true,
          decoration: InputDecoration(labelText: label),
          items: <DropdownMenuItem<String>>[
            for (final FilterOption o in list)
              DropdownMenuItem<String>(value: o.id, child: Text(o.name)),
          ],
          onChanged: (String? id) => onChanged(
            id == null
                ? null
                : list.firstWhere((FilterOption o) => o.id == id),
          ),
        );
      },
    );
  }
}
