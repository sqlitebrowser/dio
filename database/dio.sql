--
-- PostgreSQL database dump
--

-- Dumped from database version 9.6.2
-- Dumped by pg_dump version 9.6.2


--
-- Name: plpgsql; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;


--
-- Name: EXTENSION plpgsql; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION plpgsql IS 'PL/pgSQL procedural language';


SET search_path = public, pg_catalog;

SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: database_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE database_versions (
    "verID" integer NOT NULL,
    "dbID" integer NOT NULL,
    "branchHeads" jsonb NOT NULL,
    tags jsonb,
    "minioServer" text NOT NULL,
    "minioFolder" text NOT NULL,
    "minioID" text NOT NULL
);


--
-- Name: database_versions_verID_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE "database_versions_verID_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: database_versions_verID_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE "database_versions_verID_seq" OWNED BY database_versions."verID";


--
-- Name: sqlite_databases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE sqlite_databases (
    "dbName" text NOT NULL,
    "dbDefaultBranch" text DEFAULT 'master'::text NOT NULL,
    "commitList" jsonb NOT NULL
);


--
-- Name: database_versions verID; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY database_versions ALTER COLUMN "verID" SET DEFAULT nextval('"database_versions_verID_seq"'::regclass);


--
-- Name: database_versions database_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY database_versions
    ADD CONSTRAINT database_versions_pkey PRIMARY KEY ("verID");


--
-- Name: sqlite_databases sqlite_databases_pkey; Type: CONSTRAINT; Schema: public; Owner: dbhub
--

ALTER TABLE ONLY sqlite_databases
    ADD CONSTRAINT sqlite_databases_pkey PRIMARY KEY ("dbName");


--
-- PostgreSQL database dump complete
--

