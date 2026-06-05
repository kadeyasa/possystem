--
-- PostgreSQL database dump
--

-- Dumped from database version 15.4
-- Dumped by pg_dump version 17.4

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: pg_database_owner
--

CREATE SCHEMA public;


ALTER SCHEMA public OWNER TO pg_database_owner;

--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: pg_database_owner
--

COMMENT ON SCHEMA public IS 'standard public schema';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: tblaccount_mappings; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblaccount_mappings (
    id bigint NOT NULL,
    outlet_id bigint DEFAULT 0 NOT NULL,
    account_id character varying(10) NOT NULL,
    transaction_type character varying(50) NOT NULL,
    purpose character varying(50) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.tblaccount_mappings OWNER TO postgres;

--
-- Name: tblaccount_mappings_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblaccount_mappings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblaccount_mappings_id_seq OWNER TO postgres;

--
-- Name: tblaccount_mappings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblaccount_mappings_id_seq OWNED BY public.tblaccount_mappings.id;


--
-- Name: tblaccounts; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblaccounts (
    id character varying(10) NOT NULL,
    name character varying(100) NOT NULL,
    category character varying(50),
    is_active boolean DEFAULT true,
    outlet_id bigint,
    transaction_type character varying(50),
    purpose character varying(50)
);


ALTER TABLE public.tblaccounts OWNER TO postgres;

--
-- Name: tblbalance; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblbalance (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    balance numeric,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


ALTER TABLE public.tblbalance OWNER TO postgres;

--
-- Name: tblbalance_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblbalance_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblbalance_id_seq OWNER TO postgres;

--
-- Name: tblbalance_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblbalance_id_seq OWNED BY public.tblbalance.id;


--
-- Name: tblbalancehistories; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblbalancehistories (
    id bigint NOT NULL,
    balance_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    debit numeric,
    credit numeric,
    remarks character varying(200)
);


ALTER TABLE public.tblbalancehistories OWNER TO postgres;

--
-- Name: tblbalancehistories_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblbalancehistories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblbalancehistories_id_seq OWNER TO postgres;

--
-- Name: tblbalancehistories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblbalancehistories_id_seq OWNED BY public.tblbalancehistories.id;


--
-- Name: tblcategories; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblcategories (
    id bigint NOT NULL,
    outlet_id bigint,
    category_code character varying(20),
    category_name character varying(200),
    status boolean,
    icon character varying(100)
);


ALTER TABLE public.tblcategories OWNER TO postgres;

--
-- Name: tblcategories_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblcategories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblcategories_id_seq OWNER TO postgres;

--
-- Name: tblcategories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblcategories_id_seq OWNED BY public.tblcategories.id;


--
-- Name: tblcustomers; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblcustomers (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    name character varying(200),
    address character varying(200),
    email character varying(200),
    telp character varying(20),
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


ALTER TABLE public.tblcustomers OWNER TO postgres;

--
-- Name: tblcostumers_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblcostumers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblcostumers_id_seq OWNER TO postgres;

--
-- Name: tblcostumers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblcostumers_id_seq OWNED BY public.tblcustomers.id;


--
-- Name: tbldeposit; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tbldeposit (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    amount numeric,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    status integer
);


ALTER TABLE public.tbldeposit OWNER TO postgres;

--
-- Name: tbldeposit_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tbldeposit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tbldeposit_id_seq OWNER TO postgres;

--
-- Name: tbldeposit_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tbldeposit_id_seq OWNED BY public.tbldeposit.id;


--
-- Name: tblinventory_ledger; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblinventory_ledger (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    product_id bigint NOT NULL,
    variant_id bigint,
    movement_type character varying(32) NOT NULL,
    reference_type character varying(32) NOT NULL,
    reference_id bigint NOT NULL,
    quantity_in integer DEFAULT 0 NOT NULL,
    quantity_out integer DEFAULT 0 NOT NULL,
    stock_before integer DEFAULT 0 NOT NULL,
    stock_after integer DEFAULT 0 NOT NULL,
    unit_cost numeric DEFAULT 0 NOT NULL,
    total_cost numeric DEFAULT 0 NOT NULL,
    notes character varying(200),
    created_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.tblinventory_ledger OWNER TO postgres;

--
-- Name: tblinventory_ledger_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblinventory_ledger_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblinventory_ledger_id_seq OWNER TO postgres;

--
-- Name: tblinventory_ledger_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblinventory_ledger_id_seq OWNED BY public.tblinventory_ledger.id;


--
-- Name: tbljournal_entries; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tbljournal_entries (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    reference character varying(200),
    description character varying(200),
    entry_date date,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


ALTER TABLE public.tbljournal_entries OWNER TO postgres;

--
-- Name: tbljournal_entries_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tbljournal_entries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tbljournal_entries_id_seq OWNER TO postgres;

--
-- Name: tbljournal_entries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tbljournal_entries_id_seq OWNED BY public.tbljournal_entries.id;


--
-- Name: tbljournal_lines; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tbljournal_lines (
    id bigint NOT NULL,
    journal_entry_id bigint NOT NULL,
    account_id character varying(10),
    debit numeric,
    credit numeric,
    description character varying(200)
);


ALTER TABLE public.tbljournal_lines OWNER TO postgres;

--
-- Name: tbljournal_lines_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tbljournal_lines_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tbljournal_lines_id_seq OWNER TO postgres;

--
-- Name: tbljournal_lines_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tbljournal_lines_id_seq OWNED BY public.tbljournal_lines.id;


--
-- Name: tblmastermetode; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblmastermetode (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    description character varying(200),
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


ALTER TABLE public.tblmastermetode OWNER TO postgres;

--
-- Name: tblmasterwaktu; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblmasterwaktu (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    description character varying(200),
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


ALTER TABLE public.tblmasterwaktu OWNER TO postgres;

--
-- Name: tblmasterwaktu_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblmasterwaktu_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblmasterwaktu_id_seq OWNER TO postgres;

--
-- Name: tblmasterwaktu_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblmasterwaktu_id_seq OWNED BY public.tblmasterwaktu.id;


--
-- Name: tbloperational_expenses; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tbloperational_expenses (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    expense_date timestamp without time zone NOT NULL,
    expense_category character varying(64) NOT NULL,
    account_purpose character varying(64),
    reference_no character varying(200),
    payment_method character varying(64) NOT NULL,
    amount numeric DEFAULT 0 NOT NULL,
    vendor_name character varying(200),
    status character varying(16) DEFAULT 'posted'::character varying NOT NULL,
    journal_entry_id bigint,
    accounting_sync_status character varying(16),
    accounting_sync_error text,
    accounting_synced_at timestamp without time zone,
    accounting_idempotency_key text,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    deleted_at timestamp without time zone
);


ALTER TABLE public.tbloperational_expenses OWNER TO postgres;

--
-- Name: tbloperational_expenses_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tbloperational_expenses_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tbloperational_expenses_id_seq OWNER TO postgres;

--
-- Name: tbloperational_expenses_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tbloperational_expenses_id_seq OWNED BY public.tbloperational_expenses.id;


--
-- Name: tbloutletfee; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tbloutletfee (
    id bigint NOT NULL,
    outlet_id bigint,
    fee_setting numeric
);


ALTER TABLE public.tbloutletfee OWNER TO postgres;

--
-- Name: tbloutletfee_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tbloutletfee_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tbloutletfee_id_seq OWNER TO postgres;

--
-- Name: tbloutletfee_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tbloutletfee_id_seq OWNED BY public.tbloutletfee.id;


--
-- Name: tblpos_approval_requests; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblpos_approval_requests (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    request_type character varying(32) NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    transaction_id bigint NOT NULL,
    refund_id bigint,
    request_total numeric DEFAULT 0 NOT NULL,
    item_count integer DEFAULT 0 NOT NULL,
    reason character varying(255) NOT NULL,
    request_note text,
    request_payload text NOT NULL,
    requested_by_user_id text,
    requested_by_actor_type character varying(32),
    requested_by_name character varying(255),
    requested_at timestamp without time zone DEFAULT now() NOT NULL,
    reviewed_by_user_id text,
    reviewed_by_actor_type character varying(32),
    reviewed_by_name character varying(255),
    review_note text,
    reviewed_at timestamp without time zone,
    approved_at timestamp without time zone,
    rejected_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.tblpos_approval_requests OWNER TO postgres;

--
-- Name: tblpos_approval_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblpos_approval_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblpos_approval_requests_id_seq OWNER TO postgres;

--
-- Name: tblpos_approval_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblpos_approval_requests_id_seq OWNED BY public.tblpos_approval_requests.id;


--
-- Name: tblpos_order_draft_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblpos_order_draft_items (
    id bigint NOT NULL,
    order_draft_id bigint,
    product_id bigint,
    product_name text,
    quantity bigint,
    unit_price numeric,
    total numeric,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    variant_id bigint
);


ALTER TABLE public.tblpos_order_draft_items OWNER TO postgres;

--
-- Name: tblpos_order_draft_items_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblpos_order_draft_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblpos_order_draft_items_id_seq OWNER TO postgres;

--
-- Name: tblpos_order_draft_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblpos_order_draft_items_id_seq OWNED BY public.tblpos_order_draft_items.id;


--
-- Name: tblpos_order_drafts; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblpos_order_drafts (
    id bigint NOT NULL,
    outlet_id bigint,
    transaction_id bigint,
    cashier_id bigint,
    cashier_name text,
    customer_name text,
    customer_phone text,
    order_label text,
    table_label text,
    service_mode text DEFAULT 'dine_in'::text,
    source text DEFAULT 'cashier_hold'::text,
    status text DEFAULT 'open'::text,
    note text,
    subtotal numeric,
    discount_percent numeric,
    discount numeric,
    tax_percent numeric,
    tax numeric,
    total numeric,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


ALTER TABLE public.tblpos_order_drafts OWNER TO postgres;

--
-- Name: tblpos_order_drafts_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblpos_order_drafts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblpos_order_drafts_id_seq OWNER TO postgres;

--
-- Name: tblpos_order_drafts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblpos_order_drafts_id_seq OWNED BY public.tblpos_order_drafts.id;


--
-- Name: tblproduct_recipes; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblproduct_recipes (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    product_id bigint NOT NULL,
    ingredient_product_id bigint NOT NULL,
    quantity_required integer NOT NULL,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    deleted_at timestamp without time zone
);


ALTER TABLE public.tblproduct_recipes OWNER TO postgres;

--
-- Name: tblproduct_recipes_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblproduct_recipes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblproduct_recipes_id_seq OWNER TO postgres;

--
-- Name: tblproduct_recipes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblproduct_recipes_id_seq OWNED BY public.tblproduct_recipes.id;


--
-- Name: tblproducts; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblproducts (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    category_id bigint NOT NULL,
    code character varying(10) NOT NULL,
    name character varying(200) NOT NULL,
    price numeric,
    last_purchase_price numeric,
    stock integer,
    image_url text,
    is_active boolean,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    satuan character varying(20),
    item_type character varying(32)
);


ALTER TABLE public.tblproducts OWNER TO postgres;

--
-- Name: tblproducts_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblproducts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblproducts_id_seq OWNER TO postgres;

--
-- Name: tblproducts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblproducts_id_seq OWNED BY public.tblproducts.id;


--
-- Name: tblpurchase_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblpurchase_items (
    id bigint NOT NULL,
    purchase_id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity integer,
    purchase_price numeric,
    total numeric,
    variant bigint
);


ALTER TABLE public.tblpurchase_items OWNER TO postgres;

--
-- Name: tblpurchase_items_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblpurchase_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblpurchase_items_id_seq OWNER TO postgres;

--
-- Name: tblpurchase_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblpurchase_items_id_seq OWNED BY public.tblpurchase_items.id;


--
-- Name: tblpurchases; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblpurchases (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    supplier_name character varying(200),
    invoice_number character varying(200),
    purchase_date date,
    total numeric,
    note character varying(200),
    journal_entry_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    accounting_sync_status character varying(16),
    accounting_sync_error text,
    accounting_synced_at timestamp without time zone,
    accounting_idempotency_key text,
    payment_method character varying(64),
    due_date timestamp without time zone,
    paid_amount numeric DEFAULT 0 NOT NULL,
    outstanding_amount numeric DEFAULT 0 NOT NULL,
    payment_status character varying(16) DEFAULT 'paid'::character varying NOT NULL,
    linked_vendor_bill_id bigint
);


ALTER TABLE public.tblpurchases OWNER TO postgres;

--
-- Name: tblpurchases_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblpurchases_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblpurchases_id_seq OWNER TO postgres;

--
-- Name: tblpurchases_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblpurchases_id_seq OWNED BY public.tblpurchases.id;


--
-- Name: tblrefund_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblrefund_items (
    id bigint NOT NULL,
    refund_id bigint NOT NULL,
    product_id bigint,
    quantity integer,
    unit_price numeric,
    total numeric
);


ALTER TABLE public.tblrefund_items OWNER TO postgres;

--
-- Name: tblrefund_items_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblrefund_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblrefund_items_id_seq OWNER TO postgres;

--
-- Name: tblrefund_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblrefund_items_id_seq OWNED BY public.tblrefund_items.id;


--
-- Name: tblrefunds; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblrefunds (
    id bigint NOT NULL,
    transaction_id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    cashier_id bigint NOT NULL,
    refund_total numeric,
    note character varying(200),
    journal_entry_id bigint,
    created_at timestamp without time zone DEFAULT now(),
    accounting_sync_status character varying(16),
    accounting_sync_error text,
    accounting_synced_at timestamp without time zone,
    accounting_idempotency_key text,
    cashier_name character varying(255)
);


ALTER TABLE public.tblrefunds OWNER TO postgres;

--
-- Name: tblrefunds_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblrefunds_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblrefunds_id_seq OWNER TO postgres;

--
-- Name: tblrefunds_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblrefunds_id_seq OWNED BY public.tblrefunds.id;


--
-- Name: tblstock_adjustment_approval_requests; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblstock_adjustment_approval_requests (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    adjustment_date timestamp without time zone NOT NULL,
    stock_adjustment_id bigint,
    request_total numeric DEFAULT 0 NOT NULL,
    item_count integer DEFAULT 0 NOT NULL,
    reason character varying(64) NOT NULL,
    request_note text,
    request_payload text NOT NULL,
    requested_by_user_id text,
    requested_by_actor_type character varying(32),
    requested_by_name character varying(255),
    requested_at timestamp without time zone DEFAULT now() NOT NULL,
    reviewed_by_user_id text,
    reviewed_by_actor_type character varying(32),
    reviewed_by_name character varying(255),
    review_note text,
    reviewed_at timestamp without time zone,
    approved_at timestamp without time zone,
    rejected_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.tblstock_adjustment_approval_requests OWNER TO postgres;

--
-- Name: tblstock_adjustment_approval_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblstock_adjustment_approval_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblstock_adjustment_approval_requests_id_seq OWNER TO postgres;

--
-- Name: tblstock_adjustment_approval_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblstock_adjustment_approval_requests_id_seq OWNED BY public.tblstock_adjustment_approval_requests.id;


--
-- Name: tblstock_adjustment_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblstock_adjustment_items (
    id bigint NOT NULL,
    adjustment_id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity_delta integer NOT NULL,
    stock_before integer DEFAULT 0 NOT NULL,
    stock_after integer DEFAULT 0 NOT NULL,
    unit_cost numeric DEFAULT 0 NOT NULL,
    total_cost numeric DEFAULT 0 NOT NULL,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.tblstock_adjustment_items OWNER TO postgres;

--
-- Name: tblstock_adjustment_items_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblstock_adjustment_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblstock_adjustment_items_id_seq OWNER TO postgres;

--
-- Name: tblstock_adjustment_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblstock_adjustment_items_id_seq OWNED BY public.tblstock_adjustment_items.id;


--
-- Name: tblstock_adjustments; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblstock_adjustments (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    adjustment_date timestamp without time zone NOT NULL,
    reason character varying(64),
    status character varying(16) DEFAULT 'posted'::character varying NOT NULL,
    journal_entry_id bigint,
    accounting_sync_status character varying(16),
    accounting_sync_error text,
    accounting_synced_at timestamp without time zone,
    accounting_idempotency_key text,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    deleted_at timestamp without time zone
);


ALTER TABLE public.tblstock_adjustments OWNER TO postgres;

--
-- Name: tblstock_adjustments_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblstock_adjustments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblstock_adjustments_id_seq OWNER TO postgres;

--
-- Name: tblstock_adjustments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblstock_adjustments_id_seq OWNED BY public.tblstock_adjustments.id;


--
-- Name: tblstock_opname_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblstock_opname_items (
    id bigint NOT NULL,
    opname_id bigint NOT NULL,
    product_id bigint NOT NULL,
    system_stock integer DEFAULT 0 NOT NULL,
    actual_stock integer DEFAULT 0 NOT NULL,
    difference_qty integer DEFAULT 0 NOT NULL,
    unit_cost numeric DEFAULT 0 NOT NULL,
    total_cost numeric DEFAULT 0 NOT NULL,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.tblstock_opname_items OWNER TO postgres;

--
-- Name: tblstock_opname_items_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblstock_opname_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblstock_opname_items_id_seq OWNER TO postgres;

--
-- Name: tblstock_opname_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblstock_opname_items_id_seq OWNED BY public.tblstock_opname_items.id;


--
-- Name: tblstock_opnames; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblstock_opnames (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    opname_date timestamp without time zone NOT NULL,
    status character varying(16) DEFAULT 'posted'::character varying NOT NULL,
    journal_entry_id bigint,
    accounting_sync_status character varying(16),
    accounting_sync_error text,
    accounting_synced_at timestamp without time zone,
    accounting_idempotency_key text,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    deleted_at timestamp without time zone
);


ALTER TABLE public.tblstock_opnames OWNER TO postgres;

--
-- Name: tblstock_opnames_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblstock_opnames_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblstock_opnames_id_seq OWNER TO postgres;

--
-- Name: tblstock_opnames_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblstock_opnames_id_seq OWNED BY public.tblstock_opnames.id;


--
-- Name: tbltransaction_inventory_consumptions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tbltransaction_inventory_consumptions (
    id bigint NOT NULL,
    transaction_id bigint NOT NULL,
    sold_product_id bigint NOT NULL,
    inventory_product_id bigint NOT NULL,
    variant_id bigint,
    consumption_type character varying(32) NOT NULL,
    sold_quantity integer DEFAULT 0 NOT NULL,
    quantity_per_unit integer DEFAULT 0 NOT NULL,
    quantity_consumed integer DEFAULT 0 NOT NULL,
    unit_cost numeric DEFAULT 0 NOT NULL,
    total_cost numeric DEFAULT 0 NOT NULL,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.tbltransaction_inventory_consumptions OWNER TO postgres;

--
-- Name: tbltransaction_inventory_consumptions_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tbltransaction_inventory_consumptions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tbltransaction_inventory_consumptions_id_seq OWNER TO postgres;

--
-- Name: tbltransaction_inventory_consumptions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tbltransaction_inventory_consumptions_id_seq OWNED BY public.tbltransaction_inventory_consumptions.id;


--
-- Name: tbltransaction_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tbltransaction_items (
    id bigint NOT NULL,
    transaction_id bigint NOT NULL,
    product_id bigint,
    quantity integer,
    unit_price numeric,
    total numeric,
    variant_id bigint
);


ALTER TABLE public.tbltransaction_items OWNER TO postgres;

--
-- Name: tbltransaction_items_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tbltransaction_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tbltransaction_items_id_seq OWNER TO postgres;

--
-- Name: tbltransaction_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tbltransaction_items_id_seq OWNED BY public.tbltransaction_items.id;


--
-- Name: tbltransactions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tbltransactions (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    cashier_id bigint NOT NULL,
    total numeric,
    tax numeric,
    discount numeric,
    payment_method character varying(200),
    note character varying(200),
    journal_entry_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    status integer,
    accounting_sync_status character varying(16),
    accounting_sync_error text,
    accounting_synced_at timestamp without time zone,
    accounting_idempotency_key text,
    cashier_name character varying(255),
    document_status character varying(16) DEFAULT 'posted'::character varying NOT NULL,
    voided_at timestamp without time zone,
    void_reason text,
    void_approval_request_id bigint
);


ALTER TABLE public.tbltransactions OWNER TO postgres;

--
-- Name: tbltransactions_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tbltransactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tbltransactions_id_seq OWNER TO postgres;

--
-- Name: tbltransactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tbltransactions_id_seq OWNED BY public.tbltransactions.id;


--
-- Name: tblvariants; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblvariants (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    item_id bigint NOT NULL,
    metode character varying(100) NOT NULL,
    waktu character varying(100),
    durasi numeric,
    harga_online numeric,
    harga_offline numeric,
    created_at timestamp without time zone,
    deleted_at timestamp without time zone,
    biaya_produksi numeric
);


ALTER TABLE public.tblvariants OWNER TO postgres;

--
-- Name: tblvariants_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblvariants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblvariants_id_seq OWNER TO postgres;

--
-- Name: tblvariants_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblvariants_id_seq OWNED BY public.tblvariants.id;


--
-- Name: tblvendor_bills; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblvendor_bills (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    vendor_name character varying(200),
    bill_no character varying(200),
    bill_date timestamp without time zone NOT NULL,
    due_date timestamp without time zone,
    bill_type character varying(32) NOT NULL,
    account_purpose character varying(64),
    use_prepaid boolean DEFAULT false NOT NULL,
    subtotal numeric DEFAULT 0 NOT NULL,
    tax_amount numeric DEFAULT 0 NOT NULL,
    total_amount numeric DEFAULT 0 NOT NULL,
    paid_amount numeric DEFAULT 0 NOT NULL,
    outstanding_amount numeric DEFAULT 0 NOT NULL,
    status character varying(16) DEFAULT 'open'::character varying NOT NULL,
    journal_entry_id bigint,
    accounting_sync_status character varying(16),
    accounting_sync_error text,
    accounting_synced_at timestamp without time zone,
    accounting_idempotency_key text,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    deleted_at timestamp without time zone,
    purchase_id bigint
);


ALTER TABLE public.tblvendor_bills OWNER TO postgres;

--
-- Name: tblvendor_bills_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblvendor_bills_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblvendor_bills_id_seq OWNER TO postgres;

--
-- Name: tblvendor_bills_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblvendor_bills_id_seq OWNED BY public.tblvendor_bills.id;


--
-- Name: tblvendor_payment_allocations; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblvendor_payment_allocations (
    id bigint NOT NULL,
    payment_id bigint NOT NULL,
    vendor_bill_id bigint NOT NULL,
    allocated_amount numeric DEFAULT 0 NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.tblvendor_payment_allocations OWNER TO postgres;

--
-- Name: tblvendor_payment_allocations_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblvendor_payment_allocations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblvendor_payment_allocations_id_seq OWNER TO postgres;

--
-- Name: tblvendor_payment_allocations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblvendor_payment_allocations_id_seq OWNED BY public.tblvendor_payment_allocations.id;


--
-- Name: tblvendor_payments; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblvendor_payments (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    vendor_name character varying(200),
    payment_no character varying(200),
    payment_date timestamp without time zone NOT NULL,
    payment_method character varying(64) NOT NULL,
    amount numeric DEFAULT 0 NOT NULL,
    status character varying(16) DEFAULT 'posted'::character varying NOT NULL,
    journal_entry_id bigint,
    accounting_sync_status character varying(16),
    accounting_sync_error text,
    accounting_synced_at timestamp without time zone,
    accounting_idempotency_key text,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    deleted_at timestamp without time zone
);


ALTER TABLE public.tblvendor_payments OWNER TO postgres;

--
-- Name: tblvendor_payments_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblvendor_payments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblvendor_payments_id_seq OWNER TO postgres;

--
-- Name: tblvendor_payments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblvendor_payments_id_seq OWNED BY public.tblvendor_payments.id;


--
-- Name: tblvendors; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tblvendors (
    id bigint NOT NULL,
    outlet_id bigint NOT NULL,
    vendor_name character varying(200) NOT NULL,
    contact_name character varying(200),
    phone character varying(64),
    email character varying(200),
    address text,
    default_payment_term_days integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    note character varying(200),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    deleted_at timestamp without time zone
);


ALTER TABLE public.tblvendors OWNER TO postgres;

--
-- Name: tblvendors_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tblvendors_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblvendors_id_seq OWNER TO postgres;

--
-- Name: tblvendors_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tblvendors_id_seq OWNED BY public.tblvendors.id;


--
-- Name: tglmastermetode_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tglmastermetode_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tglmastermetode_id_seq OWNER TO postgres;

--
-- Name: tglmastermetode_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tglmastermetode_id_seq OWNED BY public.tblmastermetode.id;


--
-- Name: tblaccount_mappings id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblaccount_mappings ALTER COLUMN id SET DEFAULT nextval('public.tblaccount_mappings_id_seq'::regclass);


--
-- Name: tblbalance id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblbalance ALTER COLUMN id SET DEFAULT nextval('public.tblbalance_id_seq'::regclass);


--
-- Name: tblbalancehistories id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblbalancehistories ALTER COLUMN id SET DEFAULT nextval('public.tblbalancehistories_id_seq'::regclass);


--
-- Name: tblcategories id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblcategories ALTER COLUMN id SET DEFAULT nextval('public.tblcategories_id_seq'::regclass);


--
-- Name: tblcustomers id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblcustomers ALTER COLUMN id SET DEFAULT nextval('public.tblcostumers_id_seq'::regclass);


--
-- Name: tbldeposit id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tbldeposit ALTER COLUMN id SET DEFAULT nextval('public.tbldeposit_id_seq'::regclass);


--
-- Name: tblinventory_ledger id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblinventory_ledger ALTER COLUMN id SET DEFAULT nextval('public.tblinventory_ledger_id_seq'::regclass);


--
-- Name: tbljournal_entries id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tbljournal_entries ALTER COLUMN id SET DEFAULT nextval('public.tbljournal_entries_id_seq'::regclass);


--
-- Name: tbljournal_lines id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tbljournal_lines ALTER COLUMN id SET DEFAULT nextval('public.tbljournal_lines_id_seq'::regclass);


--
-- Name: tblmastermetode id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblmastermetode ALTER COLUMN id SET DEFAULT nextval('public.tglmastermetode_id_seq'::regclass);


--
-- Name: tblmasterwaktu id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblmasterwaktu ALTER COLUMN id SET DEFAULT nextval('public.tblmasterwaktu_id_seq'::regclass);


--
-- Name: tbloperational_expenses id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tbloperational_expenses ALTER COLUMN id SET DEFAULT nextval('public.tbloperational_expenses_id_seq'::regclass);


--
-- Name: tbloutletfee id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tbloutletfee ALTER COLUMN id SET DEFAULT nextval('public.tbloutletfee_id_seq'::regclass);


--
-- Name: tblpos_approval_requests id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblpos_approval_requests ALTER COLUMN id SET DEFAULT nextval('public.tblpos_approval_requests_id_seq'::regclass);


--
-- Name: tblpos_order_draft_items id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblpos_order_draft_items ALTER COLUMN id SET DEFAULT nextval('public.tblpos_order_draft_items_id_seq'::regclass);


--
-- Name: tblpos_order_drafts id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblpos_order_drafts ALTER COLUMN id SET DEFAULT nextval('public.tblpos_order_drafts_id_seq'::regclass);


--
-- Name: tblproduct_recipes id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblproduct_recipes ALTER COLUMN id SET DEFAULT nextval('public.tblproduct_recipes_id_seq'::regclass);


--
-- Name: tblproducts id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblproducts ALTER COLUMN id SET DEFAULT nextval('public.tblproducts_id_seq'::regclass);


--
-- Name: tblpurchase_items id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblpurchase_items ALTER COLUMN id SET DEFAULT nextval('public.tblpurchase_items_id_seq'::regclass);


--
-- Name: tblpurchases id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblpurchases ALTER COLUMN id SET DEFAULT nextval('public.tblpurchases_id_seq'::regclass);


--
-- Name: tblrefund_items id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblrefund_items ALTER COLUMN id SET DEFAULT nextval('public.tblrefund_items_id_seq'::regclass);


--
-- Name: tblrefunds id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblrefunds ALTER COLUMN id SET DEFAULT nextval('public.tblrefunds_id_seq'::regclass);


--
-- Name: tblstock_adjustment_approval_requests id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblstock_adjustment_approval_requests ALTER COLUMN id SET DEFAULT nextval('public.tblstock_adjustment_approval_requests_id_seq'::regclass);


--
-- Name: tblstock_adjustment_items id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblstock_adjustment_items ALTER COLUMN id SET DEFAULT nextval('public.tblstock_adjustment_items_id_seq'::regclass);


--
-- Name: tblstock_adjustments id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblstock_adjustments ALTER COLUMN id SET DEFAULT nextval('public.tblstock_adjustments_id_seq'::regclass);


--
-- Name: tblstock_opname_items id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblstock_opname_items ALTER COLUMN id SET DEFAULT nextval('public.tblstock_opname_items_id_seq'::regclass);


--
-- Name: tblstock_opnames id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblstock_opnames ALTER COLUMN id SET DEFAULT nextval('public.tblstock_opnames_id_seq'::regclass);


--
-- Name: tbltransaction_inventory_consumptions id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tbltransaction_inventory_consumptions ALTER COLUMN id SET DEFAULT nextval('public.tbltransaction_inventory_consumptions_id_seq'::regclass);


--
-- Name: tbltransaction_items id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tbltransaction_items ALTER COLUMN id SET DEFAULT nextval('public.tbltransaction_items_id_seq'::regclass);


--
-- Name: tbltransactions id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tbltransactions ALTER COLUMN id SET DEFAULT nextval('public.tbltransactions_id_seq'::regclass);


--
-- Name: tblvariants id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblvariants ALTER COLUMN id SET DEFAULT nextval('public.tblvariants_id_seq'::regclass);


--
-- Name: tblvendor_bills id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblvendor_bills ALTER COLUMN id SET DEFAULT nextval('public.tblvendor_bills_id_seq'::regclass);


--
-- Name: tblvendor_payment_allocations id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblvendor_payment_allocations ALTER COLUMN id SET DEFAULT nextval('public.tblvendor_payment_allocations_id_seq'::regclass);


--
-- Name: tblvendor_payments id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblvendor_payments ALTER COLUMN id SET DEFAULT nextval('public.tblvendor_payments_id_seq'::regclass);


--
-- Name: tblvendors id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tblvendors ALTER COLUMN id SET DEFAULT nextval('public.tblvendors_id_seq'::regclass);


--
-- PostgreSQL database dump complete
--

