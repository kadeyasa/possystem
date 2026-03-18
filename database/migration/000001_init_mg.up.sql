CREATE TABLE IF NOT EXISTS public.tblaccounts
(
    id character varying(10) COLLATE pg_catalog."default" NOT NULL,
    name character varying(100) COLLATE pg_catalog."default" NOT NULL,
    category character varying(50) COLLATE pg_catalog."default",
    is_active boolean DEFAULT true,
    outlet_id bigint,
    transaction_type character varying(50) COLLATE pg_catalog."default",
    purpose character varying(50) COLLATE pg_catalog."default",
    CONSTRAINT tblaccounts_pkey PRIMARY KEY (id)
)

CREATE TABLE IF NOT EXISTS public.tblcategories
(
    id bigint NOT NULL DEFAULT nextval('tblcategories_id_seq'::regclass),
    outlet_id bigint,
    category_code character varying(20) COLLATE pg_catalog."default",
    category_name character varying(200) COLLATE pg_catalog."default",
    status boolean,
    CONSTRAINT tblcategories_pkey PRIMARY KEY (id)
)


CREATE TABLE IF NOT EXISTS public.tblcustomers
(
    id bigint NOT NULL DEFAULT nextval('tblcostumers_id_seq'::regclass),
    outlet_id bigint NOT NULL,
    name character varying(200) COLLATE pg_catalog."default",
    address character varying(200) COLLATE pg_catalog."default",
    email character varying(200) COLLATE pg_catalog."default",
    telp character varying(10) COLLATE pg_catalog."default",
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    CONSTRAINT tblcostumers_pkey PRIMARY KEY (id, outlet_id)
)

CREATE TABLE IF NOT EXISTS public.tbljournal_entries
(
    id bigint NOT NULL DEFAULT nextval('tbljournal_entries_id_seq'::regclass),
    outlet_id bigint NOT NULL,
    reference character varying(200) COLLATE pg_catalog."default",
    description character varying(200) COLLATE pg_catalog."default",
    entry_date date,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    CONSTRAINT tbljournal_entries_pkey PRIMARY KEY (id)
)

CREATE TABLE IF NOT EXISTS public.tbljournal_lines
(
    id bigint NOT NULL DEFAULT nextval('tbljournal_lines_id_seq'::regclass),
    journal_entry_id bigint NOT NULL,
    account_id character varying(10) COLLATE pg_catalog."default",
    debit numeric,
    credit numeric,
    description character varying(200) COLLATE pg_catalog."default",
    CONSTRAINT tbljournal_lines_pkey PRIMARY KEY (id),
    CONSTRAINT fk_account FOREIGN KEY (account_id)
        REFERENCES public.tblaccounts (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
)

CREATE TABLE IF NOT EXISTS public.tblproducts
(
    id bigint NOT NULL DEFAULT nextval('tblproducts_id_seq'::regclass),
    outlet_id bigint NOT NULL,
    category_id bigint NOT NULL,
    code character varying(10) COLLATE pg_catalog."default" NOT NULL,
    name character varying(200) COLLATE pg_catalog."default" NOT NULL,
    price numeric,
    last_purchase_price numeric,
    stock integer,
    image_url text COLLATE pg_catalog."default",
    is_active boolean,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    CONSTRAINT tblproducts_pkey PRIMARY KEY (id),
    CONSTRAINT fk_category FOREIGN KEY (category_id)
        REFERENCES public.tblcategories (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        NOT VALID
)

CREATE TABLE IF NOT EXISTS public.tblpurchase_items
(
    id bigint NOT NULL DEFAULT nextval('tblpurchase_items_id_seq'::regclass),
    purchase_id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity integer,
    purchase_price numeric,
    total numeric,
    CONSTRAINT tblpurchase_items_pkey PRIMARY KEY (id)
)

CREATE TABLE IF NOT EXISTS public.tblpurchases
(
    id bigint NOT NULL DEFAULT nextval('tblpurchases_id_seq'::regclass),
    outlet_id bigint NOT NULL,
    supplier_name character varying(200) COLLATE pg_catalog."default",
    invoice_number character varying(200) COLLATE pg_catalog."default",
    purchase_date date,
    total numeric,
    note character varying(200) COLLATE pg_catalog."default",
    journal_entry_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    CONSTRAINT tblpurchases_pkey PRIMARY KEY (id),
    CONSTRAINT tblpurchases_journal_entry_id_fkey FOREIGN KEY (journal_entry_id)
        REFERENCES public.tbljournal_entries (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
)
CREATE TABLE IF NOT EXISTS public.tblrefund_items
(
    id bigint NOT NULL DEFAULT nextval('tblrefund_items_id_seq'::regclass),
    refund_id bigint NOT NULL,
    product_id bigint,
    quantity integer,
    unit_price numeric,
    total numeric,
    CONSTRAINT tblrefund_items_pkey PRIMARY KEY (id),
    CONSTRAINT fk_refund_item_refund FOREIGN KEY (refund_id)
        REFERENCES public.tblrefunds (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
)

CREATE TABLE IF NOT EXISTS public.tblrefunds
(
    id bigint NOT NULL DEFAULT nextval('tblrefunds_id_seq'::regclass),
    transaction_id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    cashier_id bigint NOT NULL,
    refund_total numeric,
    note character varying(200) COLLATE pg_catalog."default",
    journal_entry_id bigint,
    created_at timestamp without time zone DEFAULT now(),
    CONSTRAINT tblrefunds_pkey PRIMARY KEY (id),
    CONSTRAINT fk_refund_transaction FOREIGN KEY (transaction_id)
        REFERENCES public.tbltransactions (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
)

CREATE TABLE IF NOT EXISTS public.tbltransaction_items
(
    id bigint NOT NULL DEFAULT nextval('tbltransaction_items_id_seq'::regclass),
    transaction_id bigint NOT NULL,
    product_id bigint,
    quantity integer,
    unit_price numeric,
    total numeric,
    CONSTRAINT tbltransaction_items_pkey PRIMARY KEY (id)
)

CREATE TABLE IF NOT EXISTS public.tbltransactions
(
    id bigint NOT NULL DEFAULT nextval('tbltransactions_id_seq'::regclass),
    outlet_id bigint NOT NULL,
    cashier_id bigint NOT NULL,
    total numeric,
    tax numeric,
    discount numeric,
    payment_method character varying(200) COLLATE pg_catalog."default",
    note character varying(200) COLLATE pg_catalog."default",
    journal_entry_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    CONSTRAINT tbltransactions_pkey PRIMARY KEY (id),
    CONSTRAINT tbltransactions_journal_entry_id_fkey FOREIGN KEY (journal_entry_id)
        REFERENCES public.tbljournal_entries (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
)

CREATE TABLE IF NOT EXISTS public.tblvariants
(
    id bigint NOT NULL DEFAULT nextval('tblvariants_id_seq'::regclass),
    outlet_id bigint NOT NULL,
    item_id bigint NOT NULL,
    metode character varying(100) COLLATE pg_catalog."default" NOT NULL,
    waktu character varying(100) COLLATE pg_catalog."default",
    durasi numeric,
    harga_online numeric,
    harga_offline numeric,
    created_at timestamp without time zone,
    deleted_at timestamp without time zone,
    biaya_produksi numeric,
    CONSTRAINT tblvariants_pkey PRIMARY KEY (id)
)

CREATE TABLE IF NOT EXISTS public.tblmasterwaktu
(
    id bigint NOT NULL DEFAULT nextval('tblmasterwaktu_id_seq'::regclass),
    outlet_id bigint NOT NULL,
    description character varying(200) COLLATE pg_catalog."default",
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    CONSTRAINT tblmasterwaktu_pkey PRIMARY KEY (id)
)

CREATE TABLE IF NOT EXISTS public.tglmastermetode
(
    id bigint NOT NULL DEFAULT nextval('tglmastermetode_id_seq'::regclass),
    outlet_id bigint NOT NULL,
    description character varying(200) COLLATE pg_catalog."default",
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    CONSTRAINT tglmastermetode_pkey PRIMARY KEY (id)
)