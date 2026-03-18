# POS Full Accounting Process

Date: 2026-03-11

Purpose:
- Menentukan proses accounting POS yang meng-cover aktivitas utama bisnis.
- Menjadi blueprint implementasi backend `possystem`.
- Menyatukan flow inventory, operasional, asset, sewa, dan hutang dalam model yang konsisten.

Scope:
- penjualan
- refund
- inventory purchase
- operational expense
- fixed asset
- sewa
- hutang usaha
- pembayaran hutang
- stock adjustment
- penyusutan asset

Integration:
- jurnal detail tetap dibuat di `possystem`
- jurnal final tetap bisa di-sync ke `apifitness`
- `apifitness` menjadi posting ledger pusat
- `possystem` tetap menyimpan dokumen operasional dan histori lokal

## 1. Prinsip Desain

Prinsip utama yang harus dipakai:
- satu aktivitas bisnis harus punya satu dokumen sumber yang jelas
- setiap dokumen sumber harus bisa menghasilkan jurnal
- inventory movement dan accounting movement tidak boleh saling lepas
- expense operasional tidak boleh dipaksa lewat purchase inventory
- asset tidak boleh dianggap expense langsung jika manfaatnya lebih dari satu periode
- hutang harus dicatat saat kewajiban muncul, bukan hanya saat dibayar
- sewa bisa dicatat sebagai prepaid atau expense periodik, tergantung kebijakan bisnis

## 2. Domain Accounting Yang Harus Di-cover

Sistem accounting POS perlu meng-cover minimal:
- `assets`
- `liabilities`
- `equity`
- `revenue`
- `cost_of_goods_sold`
- `operating_expenses`

Rincian utama:

### 2.1 Assets

Current assets:
- cash
- bank
- raw material inventory
- merchandise inventory
- prepaid rent
- prepaid expense
- receivable jika nanti dibutuhkan

Fixed assets:
- equipment
- kitchen equipment
- furniture
- computer / POS hardware
- vehicle jika relevan

Contra asset:
- accumulated depreciation

### 2.2 Liabilities

Minimal liabilities:
- accounts payable / hutang usaha
- accrued expense
- rent payable jika ada sewa yang belum dibayar
- tax payable
- customer deposit jika nanti dipakai
- outlet fee payable jika model bisnis butuh pemisahan

### 2.3 Revenue

Minimal revenue buckets:
- product sales
- service sales
- other operating income jika ada

### 2.4 Expense

Minimal expense buckets:
- COGS
- rent expense
- utilities expense
- salary / wage expense
- supplies expense
- packaging expense
- maintenance expense
- depreciation expense
- marketing expense
- transport expense
- admin / bank charge

## 3. Chart of Accounts Yang Disarankan

Nomor akun contoh:

Assets:
- `110` Cash
- `111` Bank
- `120` Inventory Raw Material
- `121` Inventory Merchandise
- `130` Prepaid Rent
- `131` Prepaid Expense
- `150` Equipment
- `151` Furniture
- `159` Accumulated Depreciation

Liabilities:
- `210` Accounts Payable
- `211` Accrued Expense
- `212` Rent Payable
- `213` Tax Payable

Equity:
- `310` Owner Capital
- `320` Retained Earnings

Revenue:
- `410` Product Sales
- `411` Service Sales
- `412` Other Income

COGS:
- `510` COGS Merchandise
- `511` COGS Production

Operating expenses:
- `610` Rent Expense
- `611` Utilities Expense
- `612` Supplies Expense
- `613` Packaging Expense
- `614` Maintenance Expense
- `615` Salary Expense
- `616` Depreciation Expense
- `617` Marketing Expense
- `618` Admin and Bank Charge

Catatan:
- nomor akun hanya contoh
- implementasi akhir bisa menyesuaikan kebijakan outlet

## 4. Dokumen Sumber Yang Perlu Ada

Backend POS ke depan perlu punya dokumen sumber berikut:
- `pos_sale`
- `pos_refund`
- `inventory_purchase`
- `operational_expense`
- `vendor_bill`
- `vendor_payment`
- `fixed_asset_purchase`
- `rent_contract`
- `rent_invoice`
- `rent_payment`
- `stock_adjustment`
- `stock_opname`
- `depreciation_run`
- `recipe_consumption`

## 5. Flow Accounting Per Aktivitas

### 5.1 Penjualan barang jual langsung

Dokumen:
- `pos_sale`

Jurnal:
- debit cash / bank / settlement account
- credit product sales

Jika ada pajak:
- credit tax payable

Jika ada discount:
- debit sales discount atau contra revenue

Jika item stok:
- debit COGS
- credit inventory merchandise

Jika fee outlet dibebankan:
- debit expense fee outlet
- credit liability / outlet balance

### 5.2 Penjualan service

Dokumen:
- `pos_sale`

Jurnal:
- debit cash / bank
- credit service sales

Tidak ada COGS inventory jika memang murni jasa.

### 5.3 Penjualan produk racikan / production on sale

Dokumen:
- `pos_sale`
- `recipe_consumption`

Jurnal revenue:
- debit cash / bank
- credit product sales

Jurnal cost:
- debit COGS production
- credit inventory raw material

Ledger stok:
- bahan baku berkurang per recipe

### 5.4 Refund

Dokumen:
- `pos_refund`

Jurnal:
- debit sales return / sales reversal
- credit cash / bank / receivable settlement

Jika item stok kembali:
- debit inventory
- credit COGS

Wajib ada validasi:
- item harus berasal dari sale asal
- qty refund tidak boleh melebihi qty tersisa

### 5.5 Purchase inventory

Dokumen:
- `inventory_purchase`

Jika bayar tunai:
- debit inventory raw material / inventory merchandise
- credit cash / bank

Jika kredit:
- debit inventory raw material / inventory merchandise
- credit accounts payable

Ledger stok:
- qty masuk ke inventory ledger

### 5.6 Operational expense

Dokumen:
- `operational_expense`

Contoh:
- listrik
- air
- internet
- kebersihan
- packaging non-stock
- ATK
- maintenance kecil

Jika bayar tunai:
- debit expense account
- credit cash / bank

Jika belum dibayar:
- debit expense account
- credit accrued expense / accounts payable

Catatan:
- flow ini harus terpisah dari purchase inventory

### 5.7 Vendor bill / hutang usaha

Dokumen:
- `vendor_bill`

Digunakan saat vendor mengirim tagihan yang belum dibayar.

Contoh:
- pembelian stok kredit
- jasa maintenance kredit
- tagihan vendor packaging

Jurnal saat bill dibuat:
- debit inventory atau expense
- credit accounts payable

Status minimum:
- `draft`
- `open`
- `partial_paid`
- `paid`
- `void`

### 5.8 Vendor payment / pembayaran hutang

Dokumen:
- `vendor_payment`

Jurnal:
- debit accounts payable
- credit cash / bank

Aturan:
- satu payment bisa mengalokasikan ke satu atau beberapa bill
- perlu histori saldo hutang per bill

### 5.9 Fixed asset purchase

Dokumen:
- `fixed_asset_purchase`

Contoh:
- beli freezer
- beli mesin kopi
- beli komputer kasir
- beli meja kursi operasional

Jika bayar tunai:
- debit fixed asset account
- credit cash / bank

Jika kredit:
- debit fixed asset account
- credit accounts payable

Tidak boleh langsung dianggap expense kecuali nilainya kecil dan memang kebijakan bisnis mengizinkan.

### 5.10 Penyusutan asset

Dokumen:
- `depreciation_run`

Jurnal periodik:
- debit depreciation expense
- credit accumulated depreciation

Aturan:
- asset punya `acquisition_cost`
- asset punya `useful_life_months`
- asset punya `salvage_value` opsional
- sistem bisa generate jurnal bulanan

### 5.11 Sewa

Ada dua model yang perlu didukung.

Model A, sewa dibayar per periode dan langsung dibebankan:
- debit rent expense
- credit cash / bank atau rent payable

Model B, sewa dibayar di muka untuk beberapa bulan:
- saat bayar:
  - debit prepaid rent
  - credit cash / bank
- saat amortisasi bulanan:
  - debit rent expense
  - credit prepaid rent

Karena kebutuhan bisnis outlet bisa berbeda, sistem perlu mendukung dua-duanya.

Dokumen yang disarankan:
- `rent_contract`
- `rent_payment`
- `rent_amortization`

### 5.12 Stock adjustment

Dokumen:
- `stock_adjustment`

Kasus:
- barang rusak
- hilang
- koreksi salah hitung
- opname selisih

Jika stok turun:
- debit inventory loss / adjustment expense
- credit inventory

Jika stok naik:
- debit inventory
- credit inventory gain / adjustment income atau akun penyeimbang sesuai kebijakan

## 6. Tabel Baru Yang Disarankan

### 6.1 Inventory tables

- `tblinventory_ledger`
- `tblproduct_recipes`
- `tblstock_adjustments`
- `tblstock_adjustment_items`
- `tblstock_opnames`
- `tblstock_opname_items`

### 6.2 Operational expense tables

- `tbloperational_expenses`
- `tbloperational_expense_items`

### 6.3 Payables tables

- `tblvendor_bills`
- `tblvendor_bill_items`
- `tblvendor_payments`
- `tblvendor_payment_allocations`

### 6.4 Asset tables

- `tblfixed_assets`
- `tblfixed_asset_depreciations`
- `tblfixed_asset_disposals`

### 6.5 Rent tables

- `tblrent_contracts`
- `tblrent_invoices`
- `tblrent_payments`
- `tblrent_amortizations`

## 7. Kolom Inti Per Tabel

### 7.1 `tblvendor_bills`

Minimal kolom:
- `id`
- `outlet_id`
- `vendor_name`
- `bill_no`
- `bill_date`
- `due_date`
- `bill_type`
- `subtotal`
- `tax_amount`
- `total_amount`
- `paid_amount`
- `outstanding_amount`
- `status`
- `journal_entry_id`
- `accounting_sync_status`
- `accounting_sync_error`
- `accounting_synced_at`
- `accounting_idempotency_key`
- `notes`

`bill_type`:
- `inventory`
- `expense`
- `asset`
- `rent`

### 7.2 `tblvendor_payments`

Minimal kolom:
- `id`
- `outlet_id`
- `payment_no`
- `payment_date`
- `payment_method`
- `amount`
- `journal_entry_id`
- `notes`

### 7.3 `tblvendor_payment_allocations`

Minimal kolom:
- `id`
- `payment_id`
- `vendor_bill_id`
- `allocated_amount`

### 7.4 `tblfixed_assets`

Minimal kolom:
- `id`
- `outlet_id`
- `asset_code`
- `asset_name`
- `asset_category`
- `purchase_date`
- `acquisition_cost`
- `salvage_value`
- `useful_life_months`
- `depreciation_method`
- `expense_account_id`
- `asset_account_id`
- `accum_depr_account_id`
- `status`
- `journal_entry_id`

### 7.5 `tblrent_contracts`

Minimal kolom:
- `id`
- `outlet_id`
- `landlord_name`
- `contract_no`
- `start_date`
- `end_date`
- `payment_cycle`
- `amount_per_cycle`
- `prepaid_enabled`
- `expense_account_id`
- `prepaid_account_id`
- `payable_account_id`
- `status`

## 8. Accounting Source Type Yang Harus Didukung

`possystem` sebaiknya menambah source type sync ke `apifitness`:
- `pos_sale`
- `pos_refund`
- `pos_inventory_purchase`
- `pos_operational_expense`
- `pos_vendor_bill`
- `pos_vendor_payment`
- `pos_asset_purchase`
- `pos_rent_payment`
- `pos_rent_amortization`
- `pos_depreciation`
- `pos_stock_adjustment`
- `pos_recipe_consumption`

## 9. Status Yang Disarankan

### 9.1 Common document status

- `draft`
- `posted`
- `void`
- `cancelled`

### 9.2 Bill status

- `draft`
- `open`
- `partial_paid`
- `paid`
- `overdue`
- `void`

### 9.3 Asset status

- `active`
- `disposed`
- `written_off`

### 9.4 Rent contract status

- `draft`
- `active`
- `expired`
- `terminated`

## 10. Hubungan Inventory dan Accounting

Aturan inti:
- tidak semua dokumen accounting memengaruhi stok
- tidak semua mutasi stok langsung expense

Contoh:
- inventory purchase: stock iya, accounting iya
- operational expense: stock tidak, accounting iya
- asset purchase: stock tidak, accounting iya
- sale merchandise: stock iya, accounting iya
- sale service: stock tidak, accounting iya
- depreciation: stock tidak, accounting iya
- stock opname: stock iya, accounting iya jika ada nilai selisih

## 11. Prioritas Implementasi

### Step 1

Implementasi paling penting lebih dulu:
- `item_type`
- validasi stok sale
- validasi refund
- `operational_expense`
- `vendor_bill`
- `vendor_payment`

### Step 2

- `inventory_ledger`
- `stock_adjustment`
- `stock_opname`
- report revenue berbasis akun

### Step 3

- `fixed_assets`
- `depreciation_run`
- `rent_contract`
- `rent_payment`
- `rent_amortization`

### Step 4

- `product_recipes`
- raw material consumption
- improved COGS

## 12. Rekomendasi Teknis Untuk Backend Sekarang

Karena `possystem` sudah punya:
- account mapping
- local journal
- accounting sync

maka pendekatan terbaik adalah:
- pertahankan pola `document -> local journal -> sync apifitness`
- tambah document type baru, bukan memaksa semua lewat sale atau purchase
- semua dokumen baru wajib punya:
  - `journal_entry_id`
  - `accounting_sync_status`
  - `accounting_sync_error`
  - `accounting_synced_at`
  - `accounting_idempotency_key`

## 13. Catatan Penting

Jangan lakukan ini:
- mencatat asset sebagai expense biasa tanpa aturan threshold
- mencatat expense operasional lewat purchase inventory
- mengandalkan `reference = Revenue` untuk laporan revenue jangka panjang
- tetap memakai `tblproducts.stock` sebagai satu-satunya sumber audit stok

## 14. Next Action Setelah Dokumen Ini

Urutan kerja yang paling masuk akal:

1. tambah desain tabel dan model untuk `operational_expense`, `vendor_bill`, `vendor_payment`
2. tambah source type accounting baru di sync service
3. tambah validasi stok sale dan refund
4. tambah `item_type`
5. baru lanjut ke `inventory_ledger`
6. setelah itu lanjut ke `fixed_assets`, `rent`, dan `depreciation`

Dokumen ini adalah blueprint proses accounting POS penuh per 2026-03-11.
