SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: database_files; Type: TABLE; Schema: public; Owner: dbhub
--

CREATE TABLE database_files (
    sha256 text NOT NULL,
    "minioServer" text NOT NULL,
    "minioFolder" text NOT NULL,
    "minioID" text NOT NULL
);


--
-- Name: sqlite_databases; Type: TABLE; Schema: public; Owner: dbhub
--

CREATE TABLE sqlite_databases (
    "dbName" text NOT NULL,
    "dbDefaultBranch" text DEFAULT 'master'::text NOT NULL,
    "commitList" jsonb NOT NULL,
    "branchHeads" jsonb,
    tags jsonb
);


--
-- Name: database_files database_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: dbhub
--

ALTER TABLE ONLY database_files
    ADD CONSTRAINT database_versions_pkey PRIMARY KEY (sha256);


--
-- Name: sqlite_databases sqlite_databases_pkey; Type: CONSTRAINT; Schema: public; Owner: dbhub
--

ALTER TABLE ONLY sqlite_databases
    ADD CONSTRAINT sqlite_databases_pkey PRIMARY KEY ("dbName");
