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

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: tblaccount_mappings; Type: TABLE; Schema: public; Owner: kadeyasa
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


ALTER TABLE public.tblaccount_mappings OWNER TO kadeyasa;

--
-- Name: tblaccount_mappings_id_seq; Type: SEQUENCE; Schema: public; Owner: kadeyasa
--

CREATE SEQUENCE public.tblaccount_mappings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tblaccount_mappings_id_seq OWNER TO kadeyasa;

--
-- Name: tblaccount_mappings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: kadeyasa
--

ALTER SEQUENCE public.tblaccount_mappings_id_seq OWNED BY public.tblaccount_mappings.id;


--
-- Name: tblaccount_mappings id; Type: DEFAULT; Schema: public; Owner: kadeyasa
--

ALTER TABLE ONLY public.tblaccount_mappings ALTER COLUMN id SET DEFAULT nextval('public.tblaccount_mappings_id_seq'::regclass);


--
-- Data for Name: tblaccount_mappings; Type: TABLE DATA; Schema: public; Owner: kadeyasa
--

COPY public.tblaccount_mappings (id, outlet_id, account_id, transaction_type, purpose, is_active, created_at, updated_at) FROM stdin;
1	0	1101	purchase	inventory	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
2	0	1106	sale	bayarnanti	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
3	0	1100	sale	cash	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
19	0	4105	sale	discount	t	2026-03-19 15:34:34.67644	2026-03-19 15:34:34.67644
4	0	1104	sale	qris	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
5	0	4100	sale	sales	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
6	0	1105	sale	transfer	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
7	14	1101	purchase	inventory	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
8	14	1100	sale	cash	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
25	14	4105	sale	discount	t	2026-03-19 15:34:34.67644	2026-03-19 15:34:34.67644
9	14	1104	sale	qris	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
10	14	4100	sale	sales	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
11	14	1105	sale	transfer	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
12	17	1101	purchase	inventory	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
13	17	1106	sale	bayarnanti	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
14	17	1100	sale	cash	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
15	17	4100	sale	sales	t	2026-03-19 15:33:25.840147	2026-03-19 15:34:34.67644
\.


--
-- Name: tblaccount_mappings_id_seq; Type: SEQUENCE SET; Schema: public; Owner: kadeyasa
--

SELECT pg_catalog.setval('public.tblaccount_mappings_id_seq', 32, true);


--
-- Name: tblaccount_mappings tblaccount_mappings_pkey; Type: CONSTRAINT; Schema: public; Owner: kadeyasa
--

ALTER TABLE ONLY public.tblaccount_mappings
    ADD CONSTRAINT tblaccount_mappings_pkey PRIMARY KEY (id);


--
-- Name: idx_tblaccount_mappings_account; Type: INDEX; Schema: public; Owner: kadeyasa
--

CREATE INDEX idx_tblaccount_mappings_account ON public.tblaccount_mappings USING btree (account_id);


--
-- Name: idx_tblaccount_mappings_scope; Type: INDEX; Schema: public; Owner: kadeyasa
--

CREATE UNIQUE INDEX idx_tblaccount_mappings_scope ON public.tblaccount_mappings USING btree (outlet_id, transaction_type, purpose);


--
-- PostgreSQL database dump complete
--

