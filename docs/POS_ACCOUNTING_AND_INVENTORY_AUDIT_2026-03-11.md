# POS Accounting and Inventory Audit

Date: 2026-03-11

Scope:
- Backend audited: `possystem`
- Frontend related but not audited in depth: `kasi-pos`
- External accounting target already used by POS: `apifitness` accounting journal API

Purpose:
- Menyimpan konteks audit dan kebutuhan bisnis terbaru.
- Menjadi acuan lanjutan jika sesi terputus.
- Menentukan arah implementasi accounting dan inventory agar tidak perlu menjelaskan ulang dari awal.

Related document:
- `docs/POS_FULL_ACCOUNTING_PROCESS_2026-03-11.md`

Implementation status:
- 2026-03-11 step 1 backend implemented in `possystem`:
  - `operational_expense`
  - `vendor_bill`
  - `vendor_payment`
  - accounting sync record types for the three documents above
- 2026-03-11 inventory hardening implemented in `possystem`:
  - sale stock validation before posting
  - refund validation against original sold quantity and previous refunds
  - `inventory_ledger` schema and ledger writes for purchase, sale, and refund
  - `GET /api/inventory-ledger/` endpoint for stock movement audit
- 2026-03-11 inventory master and audit documents implemented in `possystem`:
  - `item_type` on product with default `resale_item`
  - `stock_adjustment` create/list/detail
  - `stock_opname` create/list/detail
  - accounting sync record types for `stock_adjustment` and `stock_opname`
- 2026-03-11 recipe/BOM and auto-consumption implemented in `possystem`:
  - `product_recipes` master for `finished_good`
  - auto raw material consumption on sale for products with active recipe
  - transaction inventory consumption snapshot for refund-safe reversal
  - refund inventory restoration now follows recorded sale consumption, not current recipe
  - new recipe endpoints: `POST/GET/DELETE /api/product-recipes`

## 1. Kebutuhan Bisnis Yang Sudah Disepakati

Sistem POS perlu mendukung:
- Pencatatan akuntansi `revenue` dan `operasional` secara terpisah.
- Manajemen stok untuk dua model:
  - barang jual langsung
  - bahan baku
- Kemampuan berkembang ke produk racikan/produksi yang mengonsumsi bahan baku.
- Audit trail yang rapi untuk transaksi keuangan dan pergerakan stok.

## 2. Kondisi Backend POS Saat Ini

Backend `possystem` saat ini sudah punya:
- transaksi penjualan
- refund
- purchase
- jurnal lokal (`tbljournal_entries`, `tbljournal_lines`)
- sinkronisasi jurnal ke `apifitness`
- mapping akun per outlet
- saldo outlet / fee outlet

Flow inti yang ada sekarang:
- `sale` membuat jurnal lokal, mengurangi `tblproducts.stock`, lalu sync ke `apifitness`
- `purchase` membuat jurnal lokal, menambah `tblproducts.stock`, lalu sync ke `apifitness`
- `refund` membuat jurnal lokal, menambah `tblproducts.stock`, lalu sync ke `apifitness`

Accounting sync yang tersedia saat ini hanya untuk:
- `sale`
- `refund`
- `purchase`

## 3. Temuan Audit Utama

### 3.1 Stok masih angka tunggal, belum ledger

Struktur sekarang:
- `tblproducts.stock`
- `tblproducts.last_purchase_price`

Belum ada:
- stock movement ledger
- stock adjustment
- stock opname
- waste / spoilage
- transfer antar lokasi
- audit trail mutasi stok per dokumen

Dampak:
- histori perubahan stok tidak lengkap
- sulit audit kenapa stok berubah
- sulit membedakan stok naik dari purchase, refund, atau koreksi manual

## 3.2 Belum ada pemisahan tipe item

Model produk saat ini belum membedakan:
- `raw_material`
- `resale_item`
- `finished_good`
- `service`

Dampak:
- purchase selalu diasumsikan masuk inventory
- produk non stok dan biaya operasional mudah salah posting
- sistem belum siap untuk racikan berbasis bahan baku

## 3.3 Purchase selalu dianggap inventory

Flow purchase saat ini selalu:
- debit `purchase:inventory`
- credit `purchase:cash` atau `purchase:credit`

Artinya pembelian operasional seperti:
- gas
- ATK
- kebersihan
- listrik/manual reimbursement
- packaging non-stock

akan salah dicatat sebagai persediaan jika dipaksa lewat endpoint purchase yang sekarang.

## 3.4 Flow operasional belum ada dokumen transaksi resminya

Saat ini belum ada entity / endpoint khusus untuk:
- operational expense
- petty cash out
- biaya outlet
- biaya non-inventory

Dampak:
- revenue sudah ada flow
- operasional belum punya flow baku
- laporan laba rugi tidak akan lengkap

## 3.5 Validasi stok sale belum cukup aman

Pada sale, stok langsung dikurangi dari `tblproducts.stock`.

Gap:
- belum ada validasi stok tersedia sebelum jual
- belum ada proteksi stok minus di level flow
- belum ada lock / ledger mutasi stok

Dampak:
- stok bisa minus
- audit stok bisa rusak

## 3.6 Validasi refund belum cukup aman

Refund saat ini menambah stok kembali, tetapi backend belum memverifikasi secara kuat bahwa:
- item memang pernah dijual di transaksi asal
- quantity refund tidak melebihi quantity terjual
- item yang sama belum pernah direfund berlebihan

Dampak:
- stok bisa bertambah palsu
- revenue dan COGS reversal bisa tidak akurat

## 3.7 Perhitungan cost masih sangat sederhana

Cost sekarang diambil dari:
- `variant.biaya_produksi`, jika ada
- fallback ke `product.last_purchase_price`

Belum ada:
- FIFO
- moving average
- lot cost
- recipe cost per bahan

Dampak:
- COGS hanya cocok untuk skenario sederhana
- belum cukup untuk bahan baku dan produksi

## 3.8 Revenue report belum cukup akurat secara accounting

Endpoint revenue sekarang mengambil total dari jurnal dengan reference `Revenue`.

Masalah:
- pendekatan ini tidak benar-benar mengunci ke akun `revenue`
- jurnal yang sama bisa berisi tax, discount, inventory, fee, dan akun lain
- hasil berisiko overstate / tidak presisi

## 4. Kesimpulan Audit

Sistem saat ini sudah punya pondasi:
- transaksi dasar
- jurnal lokal
- sync accounting
- account mapping

Tetapi sistem belum cukup kuat untuk kebutuhan:
- pemisahan revenue dan operasional
- inventory bahan baku
- produk racikan
- audit trail stok dan biaya yang rapi

Secara praktis:
- sistem masih cocok untuk `jual langsung` yang sederhana
- sistem belum siap untuk `inventory accounting` yang lebih serius

## 5. Rancangan Target Yang Disarankan

### 5.1 Tipe item

Tambahkan klasifikasi item di master product:
- `raw_material`
- `resale_item`
- `finished_good`
- `service`

Aturan:
- `raw_material`: dibeli, disimpan, dikonsumsi oleh recipe
- `resale_item`: dibeli lalu dijual langsung
- `finished_good`: dijual, dan jika perlu mengonsumsi bahan baku
- `service`: tidak punya stok

### 5.2 Inventory ledger

Tambahkan tabel mutasi stok, misalnya `tblinventory_ledger`.

Minimal kolom:
- `id`
- `outlet_id`
- `product_id`
- `movement_type`
- `reference_type`
- `reference_id`
- `qty_in`
- `qty_out`
- `unit_cost`
- `total_cost`
- `notes`
- `created_at`
- `created_by`

Nilai `movement_type` minimal:
- `purchase`
- `sale`
- `refund`
- `adjustment_in`
- `adjustment_out`
- `waste`
- `opname_gain`
- `opname_loss`
- `recipe_consumption`

Fungsi utama ledger:
- stok on hand dihitung dari saldo ledger
- semua perubahan stok bisa diaudit
- alasan mutasi jelas

### 5.3 Product recipe / BOM

Tambahkan tabel recipe untuk produk yang memakai bahan baku, misalnya:
- `tblproduct_recipes`
- `tbltransaction_inventory_consumptions`

Minimal kolom:
- `id`
- `outlet_id`
- `product_id`
- `ingredient_product_id`
- `qty_required`
- `uom`
- `waste_factor`

Aturan:
- hanya berlaku untuk `finished_good`
- saat produk dijual, sistem konsumsi bahan baku ke ledger
- snapshot konsumsi per transaksi harus disimpan agar refund tidak menghitung ulang dari recipe yang mungkin sudah berubah

### 5.4 Operational expense document

Tambahkan transaksi operasional terpisah, misalnya:
- `tbloperational_expenses`
- `tbloperational_expense_items` jika perlu detail multi-line

Minimal kolom header:
- `id`
- `outlet_id`
- `expense_date`
- `expense_category`
- `payment_method`
- `amount`
- `vendor_name`
- `notes`
- `journal_entry_id`
- `accounting_sync_status`
- `accounting_sync_error`
- `accounting_synced_at`
- `accounting_idempotency_key`

Contoh `expense_category`:
- `utilities`
- `maintenance`
- `supplies`
- `packaging`
- `transport`
- `rent`
- `petty_cash`
- `other`

Posting accounting:
- debit akun expense operasional
- credit cash / bank / payable

### 5.5 Pisahkan inventory purchase dan expense purchase

Purchase ke depan harus punya tipe:
- `inventory_purchase`
- `expense_purchase`

Aturan:
- `inventory_purchase` menambah stok dan debit inventory
- `expense_purchase` tidak menambah stok dan debit expense

### 5.6 Perbaikan report accounting

Report ke depan jangan berbasis `reference = Revenue`.

Gunakan pendekatan:
- revenue dihitung dari akun kategori revenue
- expense dihitung dari akun kategori expense
- COGS dihitung dari akun COGS
- gross profit dan net profit dihitung dari klasifikasi akun

## 6. Flow Target Yang Direkomendasikan

### 6.1 Jual langsung

Untuk `resale_item`:
1. sale dibuat
2. jurnal revenue dibuat
3. COGS dibuat
4. stok item dikurangi
5. ledger mencatat mutasi `sale`

### 6.2 Beli stok

Untuk `raw_material` atau `resale_item`:
1. purchase inventory dibuat
2. jurnal inventory dibuat
3. stok bertambah
4. ledger mencatat mutasi `purchase`

### 6.3 Produk racikan

Untuk `finished_good`:
1. sale dibuat
2. sistem baca recipe
3. bahan baku dikurangi sesuai recipe
4. ledger mencatat `recipe_consumption`
5. jurnal COGS dihitung dari bahan baku yang dikonsumsi

### 6.4 Refund

1. sistem validasi item refund terhadap transaksi asal
2. quantity refund tidak boleh melebihi sisa yang belum direfund
3. jurnal reversal dibuat
4. stok / ledger dibalik sesuai model item

### 6.5 Biaya operasional

1. expense dibuat dari dokumen operasional
2. tidak menambah stok jika memang bukan inventory
3. jurnal expense dibuat
4. sync ke accounting pusat

## 7. Prioritas Implementasi Yang Disarankan

### Phase 1 - Hardening sekarang

Target:
- amankan flow yang sudah ada sebelum tambah fitur besar

Scope:
- validasi stok tersedia saat sale
- validasi refund terhadap quantity terjual
- tambah endpoint operational expense
- pisahkan inventory purchase vs expense purchase
- perbaiki report revenue agar berbasis akun

### Phase 2 - Inventory foundation

Scope:
- tambah `item_type` di product
- tambah `tblinventory_ledger`
- semua sale / purchase / refund menulis ke ledger
- siapkan stock adjustment dan stock opname

### Phase 3 - Raw material and recipe

Scope:
- tambah recipe / BOM
- finished good konsumsi raw material otomatis
- cost bahan baku dipakai untuk COGS

Status:
- backend sudah diimplementasikan pada 2026-03-11
- yang tersisa adalah UI recipe/BOM di `kasi-pos` dan keputusan cost method lanjutan bila mau naik dari simple unit cost ke moving average/FIFO

### Phase 4 - Reporting and audit

Scope:
- laporan inventory movement
- laporan stock card
- laporan COGS
- laporan profit loss sederhana
- audit trail expense operasional

## 8. Keputusan Teknis Yang Masih Perlu Ditentukan

Beberapa hal belum diputuskan dan harus ditetapkan sebelum implementasi penuh:
- metode penilaian cost: `moving_average` atau `FIFO`
- apakah packaging masuk raw material atau expense
- apakah service item boleh muncul di transaksi yang sama dengan stok item
- apakah finished good disimpan sebagai stok jadi atau selalu dibuat saat terjual
- bagaimana policy waste dan selisih opname

Rekomendasi awal:
- gunakan `moving_average` dulu karena lebih sederhana daripada FIFO
- packaging yang habis karena penjualan masukkan ke `raw_material` jika ingin cost per item akurat
- item jasa tetap didukung sebagai `service` tanpa stok
- produk racikan default memakai konsumsi saat sale, bukan pre-production

## 9. File Penting Yang Sudah Diaudit

Backend sale:
- `controllers/transaction_controller.go`

Backend purchase:
- `controllers/purchase_controller.go`

Backend refund:
- `controllers/refund_controller.go`

Backend recipe/BOM:
- `controllers/product_recipe_controller.go`

Accounting sync:
- `services/accounting_sync_service.go`

Account mapping:
- `services/account_mapping_service.go`

Journal reporting:
- `controllers/journal_report_controller.go`

Model product:
- `models/product.go`

Model recipe:
- `models/product_recipe.go`

Model variant:
- `models/variant.go`

Schema awal:
- `database/migration/000001_init_mg.up.sql`

## 10. Rekomendasi Langkah Kerja Berikutnya

Urutan implementasi yang paling aman:

1. Tambah dokument model baru:
   - `item_type`
   - `operational_expense`
   - `inventory_ledger`

2. Implement Phase 1 lebih dulu:
   - validasi stok sale
   - validasi refund
   - endpoint operasional
   - perbaikan report revenue

3. Setelah flow lama aman, baru masuk Phase 2:
   - ledger stok
   - mutasi stok berbasis dokumen

4. Setelah ledger stabil, baru masuk Phase 3:
   - recipe / BOM
   - konsumsi bahan baku otomatis
   - snapshot konsumsi transaksi untuk refund

## 11. Catatan Penting Untuk Sesi Berikutnya

Jika melanjutkan pekerjaan setelah sesi terputus:
- baca dokumen ini lebih dulu
- recipe/BOM backend sudah ada; fokus berikutnya pindah ke frontend recipe/BOM atau cost method yang lebih presisi
- jangan campur expense operasional ke endpoint purchase inventory yang sekarang
- jangan mempertahankan report revenue berbasis `reference = Revenue` untuk jangka panjang

Dokumen ini adalah baseline keputusan audit per 2026-03-11.
