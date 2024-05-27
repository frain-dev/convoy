--
-- PostgreSQL database dump
--

-- Dumped from database version 15.2
-- Dumped by pg_dump version 16.3 (Postgres.app)

-- Started on 2024-05-27 10:57:02 WAT

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- TOC entry 6 (class 2615 OID 16389)
-- Name: convoy; Type: SCHEMA; Schema: -; Owner: convoy
--

CREATE ROLE convoy superuser;

CREATE SCHEMA convoy;


ALTER SCHEMA convoy OWNER TO convoy;

--
-- TOC entry 239 (class 1255 OID 17379)
-- Name: copy_rows(character varying, integer); Type: FUNCTION; Schema: convoy; Owner: convoy
--

CREATE FUNCTION convoy.copy_rows(pid character varying, dur integer) RETURNS void
    LANGUAGE plpgsql
AS $$
DECLARE
    cs CURSOR FOR
        SELECT * FROM convoy.events
        WHERE project_id = pid
          AND created_at >= NOW() - MAKE_INTERVAL(hours := dur);
    row_data RECORD;
BEGIN
    OPEN cs;
    LOOP
        FETCH cs INTO row_data;
        EXIT WHEN NOT FOUND;
        INSERT INTO convoy.events_search (id, event_type, endpoints, project_id, source_id, headers, raw, data,
                                          created_at, updated_at, deleted_at, url_query_params, idempotency_key,
                                          is_duplicate_event)
        VALUES (row_data.id, row_data.event_type, row_data.endpoints, row_data.project_id, row_data.source_id,
                row_data.headers, row_data.raw, row_data.data, row_data.created_at, row_data.updated_at,
                row_data.deleted_at, row_data.url_query_params, row_data.idempotency_key, row_data.is_duplicate_event);
    END LOOP;
    CLOSE cs;
END;
$$;


ALTER FUNCTION convoy.copy_rows(pid character varying, dur integer) OWNER TO convoy;

--
-- TOC entry 240 (class 1255 OID 17396)
-- Name: duration_to_seconds(interval); Type: FUNCTION; Schema: convoy; Owner: convoy
--

CREATE FUNCTION convoy.duration_to_seconds(duration_text interval DEFAULT '00:00:10'::interval) RETURNS integer
    LANGUAGE plpgsql
AS $$
DECLARE
    duration_interval INTERVAL;
BEGIN
    duration_interval := EXTRACT(EPOCH FROM duration_text);

    RETURN EXTRACT(EPOCH FROM duration_interval)::INTEGER;
END;
$$;


ALTER FUNCTION convoy.duration_to_seconds(duration_text interval) OWNER TO convoy;

--
-- TOC entry 241 (class 1255 OID 17397)
-- Name: seconds_to_interval(integer); Type: FUNCTION; Schema: convoy; Owner: convoy
--

CREATE FUNCTION convoy.seconds_to_interval(sec integer) RETURNS text
    LANGUAGE plpgsql
AS $$
DECLARE
    hours INT;
    minutes INT;
    seconds INT;
    result TEXT := '';
BEGIN
    -- Calculate hours, minutes, and remaining seconds
    hours := sec / 3600;
    minutes := (sec / 60) % 60;
    seconds := sec % 60;

    -- Build the result string
    IF hours > 0 THEN
        result := result || hours || 'h';
    END IF;
    IF minutes > 0 OR (hours > 0 AND seconds > 0) THEN
        result := result || minutes || 'm';
    END IF;
    IF seconds > 0 THEN
        result := result || seconds || 's';
    END IF;

    RETURN result;
END;
$$;


ALTER FUNCTION convoy.seconds_to_interval(sec integer) OWNER TO convoy;

--
-- TOC entry 253 (class 1255 OID 17408)
-- Name: take_token(text, integer, integer); Type: FUNCTION; Schema: convoy; Owner: convoy
--

CREATE FUNCTION convoy.take_token(_key text, _rate integer, _bucket_size integer) RETURNS boolean
    LANGUAGE plpgsql
AS $$
declare
    row record;
    next_min timestamptz;
    new_rate int;
begin
    select * from convoy.token_bucket where key = _key for update into row;
    next_min := now() + make_interval(secs := _bucket_size);

    -- the bucket doesn't exist yet
    if row is null then
        insert into convoy.token_bucket (key, rate, expires_at)
        SELECT _key, _rate, next_min
        WHERE NOT EXISTS (
            SELECT 1 FROM convoy.token_bucket WHERE key = _key
        );

        return true;
    end if;

    -- update the rate if it's different from what's in the db
    new_rate = case when row.rate != _rate then _rate else row.rate end;

    -- this bucket has expired, reset it
    if now() > row.expires_at then
        UPDATE convoy.token_bucket
        SET tokens = 1,
            expires_at = next_min,
            updated_at = default,
            rate = new_rate
        WHERE key = _key;
        return true;
    end if;

    -- take a token
    if row.tokens < new_rate then
        update convoy.token_bucket
        set tokens = row.tokens + 1,
            expires_at = next_min,
            updated_at = default,
            rate = new_rate
        where key = _key;
        return true;
    end if;

    -- no tokens for you sorry
    return false;
end;
$$;


ALTER FUNCTION convoy.take_token(_key text, _rate integer, _bucket_size integer) OWNER TO convoy;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- TOC entry 231 (class 1259 OID 16654)
-- Name: api_keys; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.api_keys (
                                 id character varying NOT NULL,
                                 name text NOT NULL,
                                 key_type text NOT NULL,
                                 mask_id text NOT NULL,
                                 role_type text,
                                 role_project character varying,
                                 role_endpoint text,
                                 hash text NOT NULL,
                                 salt text NOT NULL,
                                 user_id character varying,
                                 created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                 updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                 expires_at timestamp with time zone,
                                 deleted_at timestamp with time zone
);


ALTER TABLE convoy.api_keys OWNER TO convoy;

--
-- TOC entry 222 (class 1259 OID 16498)
-- Name: applications; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.applications (
                                     id character varying NOT NULL,
                                     project_id character varying NOT NULL,
                                     title text NOT NULL,
                                     support_email text,
                                     slack_webhook_url text,
                                     created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                     updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                     deleted_at timestamp with time zone
);


ALTER TABLE convoy.applications OWNER TO convoy;

--
-- TOC entry 227 (class 1259 OID 16586)
-- Name: configurations; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.configurations (
                                       id character varying NOT NULL,
                                       is_analytics_enabled text NOT NULL,
                                       is_signup_enabled boolean NOT NULL,
                                       storage_policy_type text NOT NULL,
                                       on_prem_path text,
                                       s3_bucket text,
                                       s3_access_key text,
                                       s3_secret_key text,
                                       s3_region text,
                                       s3_session_token text,
                                       s3_endpoint text,
                                       created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                       updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                       deleted_at timestamp with time zone,
                                       s3_prefix text
);


ALTER TABLE convoy.configurations OWNER TO convoy;

--
-- TOC entry 226 (class 1259 OID 16567)
-- Name: devices; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.devices (
                                id character varying NOT NULL,
                                project_id character varying NOT NULL,
                                host_name text NOT NULL,
                                status text NOT NULL,
                                created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                last_seen_at timestamp with time zone NOT NULL,
                                deleted_at timestamp with time zone
);


ALTER TABLE convoy.devices OWNER TO convoy;

--
-- TOC entry 220 (class 1259 OID 16453)
-- Name: endpoints; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.endpoints (
                                  id character varying NOT NULL,
                                  status text NOT NULL,
                                  owner_id text,
                                  description text,
                                  rate_limit integer NOT NULL,
                                  advanced_signatures boolean NOT NULL,
                                  slack_webhook_url text,
                                  support_email text,
                                  app_id text,
                                  project_id character varying NOT NULL,
                                  authentication_type text,
                                  authentication_type_api_key_header_name text,
                                  authentication_type_api_key_header_value text,
                                  secrets jsonb NOT NULL,
                                  created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                  updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                  deleted_at timestamp with time zone,
                                  http_timeout integer NOT NULL,
                                  rate_limit_duration integer NOT NULL,
                                  name text NOT NULL,
                                  url text NOT NULL
);


ALTER TABLE convoy.endpoints OWNER TO convoy;

--
-- TOC entry 234 (class 1259 OID 16712)
-- Name: event_deliveries; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.event_deliveries (
                                         id character varying NOT NULL,
                                         status text NOT NULL,
                                         description text NOT NULL,
                                         project_id character varying NOT NULL,
                                         endpoint_id character varying,
                                         event_id character varying NOT NULL,
                                         device_id character varying,
                                         subscription_id character varying NOT NULL,
                                         metadata jsonb NOT NULL,
                                         headers jsonb,
                                         attempts bytea,
                                         cli_metadata jsonb,
                                         created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                         updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                         deleted_at timestamp with time zone,
                                         url_query_params character varying,
                                         idempotency_key text,
                                         latency text,
                                         event_type text
);


ALTER TABLE convoy.event_deliveries OWNER TO convoy;

--
-- TOC entry 232 (class 1259 OID 16680)
-- Name: events; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.events (
                               id character varying NOT NULL,
                               event_type text NOT NULL,
                               endpoints text,
                               project_id character varying NOT NULL,
                               source_id character varying,
                               headers jsonb,
                               raw text NOT NULL,
                               data bytea NOT NULL,
                               created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                               updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                               deleted_at timestamp with time zone,
                               url_query_params character varying,
                               idempotency_key text,
                               is_duplicate_event boolean DEFAULT false
);


ALTER TABLE convoy.events OWNER TO convoy;

--
-- TOC entry 233 (class 1259 OID 16699)
-- Name: events_endpoints; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.events_endpoints (
                                         event_id character varying NOT NULL,
                                         endpoint_id character varying NOT NULL
);


ALTER TABLE convoy.events_endpoints OWNER TO convoy;

--
-- TOC entry 236 (class 1259 OID 17352)
-- Name: events_search; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.events_search (
                                      id character varying NOT NULL,
                                      event_type text NOT NULL,
                                      endpoints text,
                                      project_id character varying NOT NULL,
                                      source_id character varying,
                                      headers jsonb,
                                      raw text NOT NULL,
                                      data bytea NOT NULL,
                                      url_query_params character varying,
                                      idempotency_key text,
                                      is_duplicate_event boolean DEFAULT false,
                                      search_token tsvector GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, raw)) STORED,
                                      created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                      updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                      deleted_at timestamp with time zone
);


ALTER TABLE convoy.events_search OWNER TO convoy;

--
-- TOC entry 215 (class 1259 OID 16390)
-- Name: gorp_migrations; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.gorp_migrations (
                                        id text NOT NULL,
                                        applied_at timestamp with time zone
);


ALTER TABLE convoy.gorp_migrations OWNER TO convoy;

--
-- TOC entry 237 (class 1259 OID 17381)
-- Name: jobs; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.jobs (
                             id character varying NOT NULL,
                             type text NOT NULL,
                             status text NOT NULL,
                             project_id character varying NOT NULL,
                             started_at timestamp with time zone,
                             completed_at timestamp with time zone,
                             failed_at timestamp with time zone,
                             created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                             updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                             deleted_at timestamp with time zone
);


ALTER TABLE convoy.jobs OWNER TO convoy;

--
-- TOC entry 235 (class 1259 OID 16809)
-- Name: meta_events; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.meta_events (
                                    id character varying NOT NULL,
                                    event_type text NOT NULL,
                                    project_id character(26) NOT NULL,
                                    metadata jsonb NOT NULL,
                                    attempt jsonb,
                                    status text NOT NULL,
                                    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                    deleted_at timestamp with time zone
);


ALTER TABLE convoy.meta_events OWNER TO convoy;

--
-- TOC entry 223 (class 1259 OID 16512)
-- Name: organisation_invites; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.organisation_invites (
                                             id character varying NOT NULL,
                                             organisation_id character varying NOT NULL,
                                             invitee_email text NOT NULL,
                                             token text NOT NULL,
                                             role_type text NOT NULL,
                                             role_project text,
                                             role_endpoint text,
                                             status text NOT NULL,
                                             created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                             updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                             expires_at timestamp with time zone NOT NULL,
                                             deleted_at timestamp with time zone
);


ALTER TABLE convoy.organisation_invites OWNER TO convoy;

--
-- TOC entry 221 (class 1259 OID 16467)
-- Name: organisation_members; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.organisation_members (
                                             id character varying NOT NULL,
                                             role_type text NOT NULL,
                                             role_project text,
                                             role_endpoint text,
                                             user_id character varying NOT NULL,
                                             organisation_id character varying NOT NULL,
                                             created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                             updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                             deleted_at timestamp with time zone
);


ALTER TABLE convoy.organisation_members OWNER TO convoy;

--
-- TOC entry 217 (class 1259 OID 16408)
-- Name: organisations; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.organisations (
                                      id character varying NOT NULL,
                                      name text NOT NULL,
                                      owner_id character varying NOT NULL,
                                      custom_domain text,
                                      assigned_domain text,
                                      created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                      updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                      deleted_at timestamp with time zone
);


ALTER TABLE convoy.organisations OWNER TO convoy;

--
-- TOC entry 224 (class 1259 OID 16538)
-- Name: portal_links; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.portal_links (
                                     id character varying NOT NULL,
                                     project_id character varying NOT NULL,
                                     name text NOT NULL,
                                     token text NOT NULL,
                                     endpoints text,
                                     created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                     updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                     deleted_at timestamp with time zone,
                                     owner_id character varying,
                                     can_manage_endpoint boolean DEFAULT false
);


ALTER TABLE convoy.portal_links OWNER TO convoy;

--
-- TOC entry 225 (class 1259 OID 16554)
-- Name: portal_links_endpoints; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.portal_links_endpoints (
                                               portal_link_id character varying NOT NULL,
                                               endpoint_id character varying NOT NULL
);


ALTER TABLE convoy.portal_links_endpoints OWNER TO convoy;

--
-- TOC entry 218 (class 1259 OID 16422)
-- Name: project_configurations; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.project_configurations (
                                               id character varying NOT NULL,
                                               retention_policy_policy text NOT NULL,
                                               max_payload_read_size integer NOT NULL,
                                               replay_attacks_prevention_enabled boolean NOT NULL,
                                               retention_policy_enabled boolean NOT NULL,
                                               ratelimit_count integer NOT NULL,
                                               ratelimit_duration integer NOT NULL,
                                               strategy_type text NOT NULL,
                                               strategy_duration integer NOT NULL,
                                               strategy_retry_count integer NOT NULL,
                                               signature_header text NOT NULL,
                                               signature_versions jsonb NOT NULL,
                                               created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                               updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                               deleted_at timestamp with time zone,
                                               disable_endpoint boolean DEFAULT false NOT NULL,
                                               meta_events_enabled boolean DEFAULT false NOT NULL,
                                               meta_events_type text,
                                               meta_events_event_type text,
                                               meta_events_url text,
                                               meta_events_secret text,
                                               meta_events_pub_sub jsonb,
                                               search_policy text DEFAULT '720h'::text,
                                               multiple_endpoint_subscriptions boolean DEFAULT false NOT NULL,
                                               ssl_enforce_secure_endpoints boolean DEFAULT true
);


ALTER TABLE convoy.project_configurations OWNER TO convoy;

--
-- TOC entry 219 (class 1259 OID 16431)
-- Name: projects; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.projects (
                                 id character varying NOT NULL,
                                 name text NOT NULL,
                                 type text NOT NULL,
                                 logo_url text,
                                 retained_events integer DEFAULT 0,
                                 organisation_id character varying NOT NULL,
                                 project_configuration_id character(26) NOT NULL,
                                 created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                 updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                 deleted_at timestamp with time zone
);


ALTER TABLE convoy.projects OWNER TO convoy;

--
-- TOC entry 228 (class 1259 OID 16595)
-- Name: source_verifiers; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.source_verifiers (
                                         id character varying NOT NULL,
                                         type text NOT NULL,
                                         basic_username text,
                                         basic_password text,
                                         api_key_header_name text,
                                         api_key_header_value text,
                                         hmac_hash text,
                                         hmac_header text,
                                         hmac_secret text,
                                         hmac_encoding text,
                                         twitter_crc_verified_at timestamp with time zone,
                                         created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                         updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                         deleted_at timestamp with time zone
);


ALTER TABLE convoy.source_verifiers OWNER TO convoy;

--
-- TOC entry 229 (class 1259 OID 16604)
-- Name: sources; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.sources (
                                id character varying NOT NULL,
                                name text NOT NULL,
                                type text NOT NULL,
                                mask_id text NOT NULL,
                                provider text NOT NULL,
                                is_disabled boolean NOT NULL,
                                forward_headers text[],
                                project_id character varying NOT NULL,
                                source_verifier_id character(26),
                                pub_sub jsonb,
                                created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                deleted_at timestamp with time zone,
                                custom_response_body character varying,
                                custom_response_content_type character varying,
                                idempotency_keys text[],
                                body_function text,
                                header_function text
);


ALTER TABLE convoy.sources OWNER TO convoy;

--
-- TOC entry 230 (class 1259 OID 16625)
-- Name: subscriptions; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.subscriptions (
                                      id character varying NOT NULL,
                                      name text NOT NULL,
                                      type text NOT NULL,
                                      project_id character varying NOT NULL,
                                      endpoint_id character varying,
                                      device_id character varying,
                                      source_id character varying,
                                      alert_config_count integer NOT NULL,
                                      alert_config_threshold text NOT NULL,
                                      retry_config_type text NOT NULL,
                                      retry_config_duration integer NOT NULL,
                                      retry_config_retry_count integer NOT NULL,
                                      filter_config_event_types text[] NOT NULL,
                                      filter_config_filter_headers jsonb NOT NULL,
                                      filter_config_filter_body jsonb NOT NULL,
                                      rate_limit_config_count integer NOT NULL,
                                      rate_limit_config_duration integer NOT NULL,
                                      created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                      updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                      deleted_at timestamp with time zone,
                                      function text
);


ALTER TABLE convoy.subscriptions OWNER TO convoy;

--
-- TOC entry 238 (class 1259 OID 17398)
-- Name: token_bucket; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE UNLOGGED TABLE convoy.token_bucket (
                                              key text NOT NULL,
                                              rate integer NOT NULL,
                                              tokens integer DEFAULT 1,
                                              created_at timestamp with time zone DEFAULT now(),
                                              updated_at timestamp with time zone DEFAULT now(),
                                              expires_at timestamp with time zone NOT NULL
);


ALTER TABLE convoy.token_bucket OWNER TO convoy;

--
-- TOC entry 216 (class 1259 OID 16397)
-- Name: users; Type: TABLE; Schema: convoy; Owner: convoy
--

CREATE TABLE convoy.users (
                              id character varying NOT NULL,
                              first_name text NOT NULL,
                              last_name text NOT NULL,
                              email text NOT NULL,
                              password text NOT NULL,
                              email_verified boolean NOT NULL,
                              reset_password_token text,
                              email_verification_token text,
                              created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                              updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                              deleted_at timestamp with time zone,
                              reset_password_expires_at timestamp with time zone,
                              email_verification_expires_at timestamp with time zone
);


ALTER TABLE convoy.users OWNER TO convoy;

--
-- TOC entry 3640 (class 0 OID 16654)
-- Dependencies: 231
-- Data for Name: api_keys; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.api_keys VALUES ('01HYWQJJ6RFVRF4M1ND9K8W50D', 'TestProj''s default key', '', '5DadVCTi3q0RUTUZ', 'admin', '01HYWQJJ5ZH4H158E4RYHGNSDC', NULL, 'VlHjvhW75k6sfa6mgbGedVgeOCsHVSPTxUlDfWS_cvc=', 'hVUVlEi5MMo2PIT6JGr45QAU5', NULL, '2024-05-27 09:54:44.577743+00', '2024-05-27 09:54:44.577743+00', NULL, NULL);


--
-- TOC entry 3631 (class 0 OID 16498)
-- Dependencies: 222
-- Data for Name: applications; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3636 (class 0 OID 16586)
-- Dependencies: 227
-- Data for Name: configurations; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.configurations VALUES ('01HYWQGQMRSGYRKYD6NX8WEZZZ', 'true', true, 'on-prem', NULL, '', '', '', '', '', '', '2024-05-27 09:53:44.607438+00', '2024-05-27 09:53:44.607438+00', NULL, '');


--
-- TOC entry 3635 (class 0 OID 16567)
-- Dependencies: 226
-- Data for Name: devices; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3629 (class 0 OID 16453)
-- Dependencies: 220
-- Data for Name: endpoints; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.endpoints VALUES ('01HYWQMQQ19S09MPJYTCJJ7XAG', 'active', '', '', 0, false, '', '', '01HYWQMQQ19S09MPJYTCJJ7XAG', '01HYWQJJ5ZH4H158E4RYHGNSDC', '', '', '', '[{"uid": "01HYWQMQQ19S09MPJYTGCCK8MM", "value": "GI1kJ7jYSsKb_lSnZW66T1xNh", "created_at": "2024-05-27T09:55:55.745289Z", "deleted_at": null, "expires_at": null, "updated_at": "2024-05-27T09:55:55.745289Z"}]', '2024-05-27 09:55:55.762408+00', '2024-05-27 09:55:55.762408+00', NULL, 0, 0, 'TestEP', 'https://jsonplaceholder.typicode.com/todos');


--
-- TOC entry 3643 (class 0 OID 16712)
-- Dependencies: 234
-- Data for Name: event_deliveries; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3641 (class 0 OID 16680)
-- Dependencies: 232
-- Data for Name: events; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3642 (class 0 OID 16699)
-- Dependencies: 233
-- Data for Name: events_endpoints; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3645 (class 0 OID 17352)
-- Dependencies: 236
-- Data for Name: events_search; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3624 (class 0 OID 16390)
-- Dependencies: 215
-- Data for Name: gorp_migrations; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.gorp_migrations VALUES ('1677078479.sql', '2024-05-27 09:53:20.671803+00');
INSERT INTO convoy.gorp_migrations VALUES ('1677770163.sql', '2024-05-27 09:53:20.741834+00');
INSERT INTO convoy.gorp_migrations VALUES ('1679836136.sql', '2024-05-27 09:53:20.760261+00');
INSERT INTO convoy.gorp_migrations VALUES ('1681821969.sql', '2024-05-27 09:53:20.774711+00');
INSERT INTO convoy.gorp_migrations VALUES ('1684504109.sql', '2024-05-27 09:53:20.970617+00');
INSERT INTO convoy.gorp_migrations VALUES ('1684884904.sql', '2024-05-27 09:53:21.036376+00');
INSERT INTO convoy.gorp_migrations VALUES ('1684918027.sql', '2024-05-27 09:53:21.052227+00');
INSERT INTO convoy.gorp_migrations VALUES ('1684929840.sql', '2024-05-27 09:53:21.059329+00');
INSERT INTO convoy.gorp_migrations VALUES ('1685202737.sql', '2024-05-27 09:53:21.06761+00');
INSERT INTO convoy.gorp_migrations VALUES ('1686048402.sql', '2024-05-27 09:53:21.074274+00');
INSERT INTO convoy.gorp_migrations VALUES ('1686656160.sql', '2024-05-27 09:53:21.084177+00');
INSERT INTO convoy.gorp_migrations VALUES ('1692024707.sql', '2024-05-27 09:53:21.093437+00');
INSERT INTO convoy.gorp_migrations VALUES ('1692105853.sql', '2024-05-27 09:53:21.116106+00');
INSERT INTO convoy.gorp_migrations VALUES ('1692699318.sql', '2024-05-27 09:53:21.12821+00');
INSERT INTO convoy.gorp_migrations VALUES ('1693908172.sql', '2024-05-27 09:53:21.134774+00');
INSERT INTO convoy.gorp_migrations VALUES ('1698074481.sql', '2024-05-27 09:53:21.140735+00');
INSERT INTO convoy.gorp_migrations VALUES ('1698683940.sql', '2024-05-27 09:53:21.147006+00');
INSERT INTO convoy.gorp_migrations VALUES ('1704372039.sql', '2024-05-27 09:53:21.156713+00');
INSERT INTO convoy.gorp_migrations VALUES ('1705562731.sql', '2024-05-27 09:53:21.165297+00');
INSERT INTO convoy.gorp_migrations VALUES ('1705575999.sql', '2024-05-27 09:53:21.176159+00');
INSERT INTO convoy.gorp_migrations VALUES ('1708434555.sql', '2024-05-27 09:53:21.190297+00');
INSERT INTO convoy.gorp_migrations VALUES ('1709568783.sql', '2024-05-27 09:53:21.198971+00');
INSERT INTO convoy.gorp_migrations VALUES ('1709729972.sql', '2024-05-27 09:53:21.208342+00');
INSERT INTO convoy.gorp_migrations VALUES ('1710685343.sql', '2024-05-27 09:53:21.217133+00');
INSERT INTO convoy.gorp_migrations VALUES ('1710763531.sql', '2024-05-27 09:53:21.224053+00');


--
-- TOC entry 3646 (class 0 OID 17381)
-- Dependencies: 237
-- Data for Name: jobs; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3644 (class 0 OID 16809)
-- Dependencies: 235
-- Data for Name: meta_events; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3632 (class 0 OID 16512)
-- Dependencies: 223
-- Data for Name: organisation_invites; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3630 (class 0 OID 16467)
-- Dependencies: 221
-- Data for Name: organisation_members; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.organisation_members VALUES ('01HYWQJ4WBK5Q09QK4RSB1DH5E', 'super_user', NULL, NULL, '01HYWQGG4XBJZZXSHT36B3YY2Y', '01HYWQJ4TXSJJBA5PT4HN0ZXYM', '2024-05-27 09:54:30.934814+00', '2024-05-27 09:54:30.934814+00', NULL);


--
-- TOC entry 3626 (class 0 OID 16408)
-- Dependencies: 217
-- Data for Name: organisations; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.organisations VALUES ('01HYWQJ4TXSJJBA5PT4HN0ZXYM', 'TestOrg', '01HYWQGG4XBJZZXSHT36B3YY2Y', NULL, NULL, '2024-05-27 09:54:30.908069+00', '2024-05-27 09:54:30.908069+00', NULL);


--
-- TOC entry 3633 (class 0 OID 16538)
-- Dependencies: 224
-- Data for Name: portal_links; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3634 (class 0 OID 16554)
-- Dependencies: 225
-- Data for Name: portal_links_endpoints; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3627 (class 0 OID 16422)
-- Dependencies: 218
-- Data for Name: project_configurations; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.project_configurations VALUES ('01HYWQJJ69AMMWG9WXGQ3WHBXD', '', 0, false, false, 1000, 60, 'linear', 100, 10, 'X-Convoy-Signature', '[{"uid": "01HYWQGQHSXSJKK0WPPPR91S2P", "hash": "SHA256", "encoding": "hex", "created_at": "2024-05-27T09:53:44.505387Z"}]', '2024-05-27 09:54:44.55225+00', '2024-05-27 09:54:44.55225+00', NULL, false, false, '', NULL, '', '', NULL, '', false, true);


--
-- TOC entry 3628 (class 0 OID 16431)
-- Dependencies: 219
-- Data for Name: projects; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.projects VALUES ('01HYWQJJ5ZH4H158E4RYHGNSDC', 'TestProj', 'outgoing', '', 0, '01HYWQJ4TXSJJBA5PT4HN0ZXYM', '01HYWQJJ69AMMWG9WXGQ3WHBXD', '2024-05-27 09:54:44.55225+00', '2024-05-27 09:54:44.55225+00', NULL);


--
-- TOC entry 3637 (class 0 OID 16595)
-- Dependencies: 228
-- Data for Name: source_verifiers; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3638 (class 0 OID 16604)
-- Dependencies: 229
-- Data for Name: sources; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3639 (class 0 OID 16625)
-- Dependencies: 230
-- Data for Name: subscriptions; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.subscriptions VALUES ('01HYWQMQSHCZ8EEY7KW1NVDN9Y', 'TestEP''s Subscription', 'api', '01HYWQJJ5ZH4H158E4RYHGNSDC', '01HYWQMQQ19S09MPJYTCJJ7XAG', NULL, NULL, 0, '', '', 0, 0, '{*}', '{}', '{}', 0, 0, '2024-05-27 09:55:55.831953+00', '2024-05-27 09:55:55.831953+00', NULL, '');


--
-- TOC entry 3647 (class 0 OID 17398)
-- Dependencies: 238
-- Data for Name: token_bucket; Type: TABLE DATA; Schema: convoy; Owner: convoy
--



--
-- TOC entry 3625 (class 0 OID 16397)
-- Dependencies: 216
-- Data for Name: users; Type: TABLE DATA; Schema: convoy; Owner: convoy
--

INSERT INTO convoy.users VALUES ('01HYWQGG4XBJZZXSHT36B3YY2Y', 'default', 'default', 'superuser@default.com', '$2a$12$AL5ZmBweQXzlYTcGYmmpge8Dn5q0cqtYzW0qY5.WOau2TRfdPRmBa', true, '', '', '2024-05-27 09:53:36.942451+00', '2024-05-27 09:53:36.942451+00', NULL, '0001-01-01 00:00:00+00', '0001-01-01 00:00:00+00');


--
-- TOC entry 3405 (class 2606 OID 16664)
-- Name: api_keys api_keys_mask_id_key; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.api_keys
    ADD CONSTRAINT api_keys_mask_id_key UNIQUE NULLS NOT DISTINCT (mask_id, deleted_at);


--
-- TOC entry 3407 (class 2606 OID 17222)
-- Name: api_keys api_keys_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.api_keys
    ADD CONSTRAINT api_keys_pkey PRIMARY KEY (id);


--
-- TOC entry 3373 (class 2606 OID 17299)
-- Name: applications applications_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.applications
    ADD CONSTRAINT applications_pkey PRIMARY KEY (id);


--
-- TOC entry 3392 (class 2606 OID 17049)
-- Name: configurations configurations_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.configurations
    ADD CONSTRAINT configurations_pkey PRIMARY KEY (id);


--
-- TOC entry 3390 (class 2606 OID 17051)
-- Name: devices devices_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.devices
    ADD CONSTRAINT devices_pkey PRIMARY KEY (id);


--
-- TOC entry 3361 (class 2606 OID 16954)
-- Name: endpoints endpoints_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.endpoints
    ADD CONSTRAINT endpoints_pkey PRIMARY KEY (id);


--
-- TOC entry 3422 (class 2606 OID 17117)
-- Name: event_deliveries event_deliveries_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.event_deliveries
    ADD CONSTRAINT event_deliveries_pkey PRIMARY KEY (id);


--
-- TOC entry 3410 (class 2606 OID 17242)
-- Name: events events_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (id);


--
-- TOC entry 3433 (class 2606 OID 17362)
-- Name: events_search events_search_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.events_search
    ADD CONSTRAINT events_search_pkey PRIMARY KEY (id);


--
-- TOC entry 3346 (class 2606 OID 16396)
-- Name: gorp_migrations gorp_migrations_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.gorp_migrations
    ADD CONSTRAINT gorp_migrations_pkey PRIMARY KEY (id);


--
-- TOC entry 3441 (class 2606 OID 17389)
-- Name: jobs jobs_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);


--
-- TOC entry 3431 (class 2606 OID 17312)
-- Name: meta_events meta_events_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.meta_events
    ADD CONSTRAINT meta_events_pkey PRIMARY KEY (id);


--
-- TOC entry 3357 (class 2606 OID 16880)
-- Name: projects name_org_id_key; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.projects
    ADD CONSTRAINT name_org_id_key UNIQUE NULLS NOT DISTINCT (name, organisation_id, deleted_at);


--
-- TOC entry 3377 (class 2606 OID 17006)
-- Name: organisation_invites organisation_invites_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_invites
    ADD CONSTRAINT organisation_invites_pkey PRIMARY KEY (id);


--
-- TOC entry 3379 (class 2606 OID 16522)
-- Name: organisation_invites organisation_invites_token_key; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_invites
    ADD CONSTRAINT organisation_invites_token_key UNIQUE NULLS NOT DISTINCT (token, deleted_at);


--
-- TOC entry 3369 (class 2606 OID 17023)
-- Name: organisation_members organisation_members_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_members
    ADD CONSTRAINT organisation_members_pkey PRIMARY KEY (id);


--
-- TOC entry 3371 (class 2606 OID 17025)
-- Name: organisation_members organisation_members_user_id_org_id_key; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_members
    ADD CONSTRAINT organisation_members_user_id_org_id_key UNIQUE NULLS NOT DISTINCT (organisation_id, user_id, deleted_at);


--
-- TOC entry 3353 (class 2606 OID 16849)
-- Name: organisations organisations_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisations
    ADD CONSTRAINT organisations_pkey PRIMARY KEY (id);


--
-- TOC entry 3384 (class 2606 OID 17074)
-- Name: portal_links portal_links_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.portal_links
    ADD CONSTRAINT portal_links_pkey PRIMARY KEY (id);


--
-- TOC entry 3386 (class 2606 OID 16548)
-- Name: portal_links portal_links_token; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.portal_links
    ADD CONSTRAINT portal_links_token UNIQUE NULLS NOT DISTINCT (token, deleted_at);


--
-- TOC entry 3355 (class 2606 OID 17320)
-- Name: project_configurations project_configurations_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.project_configurations
    ADD CONSTRAINT project_configurations_pkey PRIMARY KEY (id);


--
-- TOC entry 3359 (class 2606 OID 16878)
-- Name: projects projects_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);


--
-- TOC entry 3394 (class 2606 OID 17333)
-- Name: source_verifiers source_verifiers_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.source_verifiers
    ADD CONSTRAINT source_verifiers_pkey PRIMARY KEY (id);


--
-- TOC entry 3399 (class 2606 OID 16614)
-- Name: sources sources_mask_id; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.sources
    ADD CONSTRAINT sources_mask_id UNIQUE NULLS NOT DISTINCT (mask_id, deleted_at);


--
-- TOC entry 3401 (class 2606 OID 17161)
-- Name: sources sources_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.sources
    ADD CONSTRAINT sources_pkey PRIMARY KEY (id);


--
-- TOC entry 3403 (class 2606 OID 17189)
-- Name: subscriptions subscriptions_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.subscriptions
    ADD CONSTRAINT subscriptions_pkey PRIMARY KEY (id);


--
-- TOC entry 3443 (class 2606 OID 17407)
-- Name: token_bucket token_bucket_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.token_bucket
    ADD CONSTRAINT token_bucket_pkey PRIMARY KEY (key);


--
-- TOC entry 3348 (class 2606 OID 16407)
-- Name: users users_email_key; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.users
    ADD CONSTRAINT users_email_key UNIQUE NULLS NOT DISTINCT (email, deleted_at);


--
-- TOC entry 3350 (class 2606 OID 16825)
-- Name: users users_pkey; Type: CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- TOC entry 3420 (class 1259 OID 17395)
-- Name: event_deliveries_event_type_1; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX event_deliveries_event_type_1 ON convoy.event_deliveries USING btree (event_type);


--
-- TOC entry 3408 (class 1259 OID 16795)
-- Name: idx_api_keys_mask_id; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_api_keys_mask_id ON convoy.api_keys USING btree (mask_id);


--
-- TOC entry 3362 (class 1259 OID 16748)
-- Name: idx_endpoints_app_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_endpoints_app_id_key ON convoy.endpoints USING btree (app_id);


--
-- TOC entry 3363 (class 1259 OID 16747)
-- Name: idx_endpoints_owner_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_endpoints_owner_id_key ON convoy.endpoints USING btree (owner_id);


--
-- TOC entry 3364 (class 1259 OID 16955)
-- Name: idx_endpoints_project_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_endpoints_project_id_key ON convoy.endpoints USING btree (project_id);


--
-- TOC entry 3423 (class 1259 OID 16762)
-- Name: idx_event_deliveries_created_at_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_event_deliveries_created_at_key ON convoy.event_deliveries USING btree (created_at);


--
-- TOC entry 3424 (class 1259 OID 16763)
-- Name: idx_event_deliveries_deleted_at_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_event_deliveries_deleted_at_key ON convoy.event_deliveries USING btree (deleted_at);


--
-- TOC entry 3425 (class 1259 OID 17118)
-- Name: idx_event_deliveries_device_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_event_deliveries_device_id_key ON convoy.event_deliveries USING btree (device_id);


--
-- TOC entry 3426 (class 1259 OID 17119)
-- Name: idx_event_deliveries_endpoint_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_event_deliveries_endpoint_id_key ON convoy.event_deliveries USING btree (endpoint_id);


--
-- TOC entry 3427 (class 1259 OID 17120)
-- Name: idx_event_deliveries_event_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_event_deliveries_event_id_key ON convoy.event_deliveries USING btree (event_id);


--
-- TOC entry 3428 (class 1259 OID 17121)
-- Name: idx_event_deliveries_project_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_event_deliveries_project_id_key ON convoy.event_deliveries USING btree (project_id);


--
-- TOC entry 3429 (class 1259 OID 16760)
-- Name: idx_event_deliveries_status_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_event_deliveries_status_key ON convoy.event_deliveries USING btree (status);


--
-- TOC entry 3411 (class 1259 OID 16754)
-- Name: idx_events_created_at_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_created_at_key ON convoy.events USING btree (created_at);


--
-- TOC entry 3412 (class 1259 OID 16755)
-- Name: idx_events_deleted_at_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_deleted_at_key ON convoy.events USING btree (deleted_at);


--
-- TOC entry 3418 (class 1259 OID 17278)
-- Name: idx_events_endpoints_endpoint_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_endpoints_endpoint_id_key ON convoy.events_endpoints USING btree (endpoint_id);


--
-- TOC entry 3419 (class 1259 OID 17277)
-- Name: idx_events_endpoints_event_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_endpoints_event_id_key ON convoy.events_endpoints USING btree (event_id);


--
-- TOC entry 3413 (class 1259 OID 17244)
-- Name: idx_events_project_id_deleted_at_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_project_id_deleted_at_key ON convoy.events USING btree (project_id, deleted_at);


--
-- TOC entry 3414 (class 1259 OID 17243)
-- Name: idx_events_project_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_project_id_key ON convoy.events USING btree (project_id);


--
-- TOC entry 3434 (class 1259 OID 17374)
-- Name: idx_events_search_created_at_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_search_created_at_key ON convoy.events_search USING btree (created_at);


--
-- TOC entry 3435 (class 1259 OID 17375)
-- Name: idx_events_search_deleted_at_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_search_deleted_at_key ON convoy.events_search USING btree (deleted_at);


--
-- TOC entry 3436 (class 1259 OID 17377)
-- Name: idx_events_search_project_id_deleted_at_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_search_project_id_deleted_at_key ON convoy.events_search USING btree (project_id, deleted_at);


--
-- TOC entry 3437 (class 1259 OID 17376)
-- Name: idx_events_search_project_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_search_project_id_key ON convoy.events_search USING btree (project_id);


--
-- TOC entry 3438 (class 1259 OID 17378)
-- Name: idx_events_search_source_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_search_source_id_key ON convoy.events_search USING btree (source_id);


--
-- TOC entry 3439 (class 1259 OID 17373)
-- Name: idx_events_search_token_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_search_token_key ON convoy.events_search USING gin (search_token);


--
-- TOC entry 3415 (class 1259 OID 17245)
-- Name: idx_events_source_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_events_source_id_key ON convoy.events USING btree (source_id);


--
-- TOC entry 3416 (class 1259 OID 17349)
-- Name: idx_idempotency_key_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_idempotency_key_key ON convoy.events USING btree (idempotency_key);


--
-- TOC entry 3374 (class 1259 OID 16794)
-- Name: idx_organisation_invites_token_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_organisation_invites_token_key ON convoy.organisation_invites USING btree (token);


--
-- TOC entry 3365 (class 1259 OID 16751)
-- Name: idx_organisation_members_deleted_at_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_organisation_members_deleted_at_key ON convoy.organisation_members USING btree (deleted_at);


--
-- TOC entry 3366 (class 1259 OID 17027)
-- Name: idx_organisation_members_organisation_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_organisation_members_organisation_id_key ON convoy.organisation_members USING btree (organisation_id);


--
-- TOC entry 3367 (class 1259 OID 17026)
-- Name: idx_organisation_members_user_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_organisation_members_user_id_key ON convoy.organisation_members USING btree (user_id);


--
-- TOC entry 3387 (class 1259 OID 17096)
-- Name: idx_portal_links_endpoints_enpdoint_id; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_portal_links_endpoints_enpdoint_id ON convoy.portal_links_endpoints USING btree (endpoint_id);


--
-- TOC entry 3388 (class 1259 OID 17095)
-- Name: idx_portal_links_endpoints_portal_link_id; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_portal_links_endpoints_portal_link_id ON convoy.portal_links_endpoints USING btree (portal_link_id);


--
-- TOC entry 3380 (class 1259 OID 17347)
-- Name: idx_portal_links_owner_id_key; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_portal_links_owner_id_key ON convoy.portal_links USING btree (owner_id);


--
-- TOC entry 3381 (class 1259 OID 17075)
-- Name: idx_portal_links_project_id; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_portal_links_project_id ON convoy.portal_links USING btree (project_id);


--
-- TOC entry 3382 (class 1259 OID 16800)
-- Name: idx_portal_links_token; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_portal_links_token ON convoy.portal_links USING btree (token);


--
-- TOC entry 3417 (class 1259 OID 17409)
-- Name: idx_project_id_on_not_deleted; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_project_id_on_not_deleted ON convoy.events USING btree (project_id) WHERE (deleted_at IS NULL);


--
-- TOC entry 3395 (class 1259 OID 16798)
-- Name: idx_sources_mask_id; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_sources_mask_id ON convoy.sources USING btree (mask_id);


--
-- TOC entry 3396 (class 1259 OID 17162)
-- Name: idx_sources_project_id; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_sources_project_id ON convoy.sources USING btree (project_id);


--
-- TOC entry 3397 (class 1259 OID 16796)
-- Name: idx_sources_source_verifier_id; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE INDEX idx_sources_source_verifier_id ON convoy.sources USING btree (source_verifier_id);


--
-- TOC entry 3375 (class 1259 OID 17345)
-- Name: organisation_invites_invitee_email; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE UNIQUE INDEX organisation_invites_invitee_email ON convoy.organisation_invites USING btree (organisation_id, invitee_email, deleted_at) NULLS NOT DISTINCT;


--
-- TOC entry 3351 (class 1259 OID 16792)
-- Name: organisations_custom_domain; Type: INDEX; Schema: convoy; Owner: convoy
--

CREATE UNIQUE INDEX organisations_custom_domain ON convoy.organisations USING btree (custom_domain, assigned_domain) WHERE (deleted_at IS NULL);


--
-- TOC entry 3466 (class 2606 OID 16981)
-- Name: api_keys api_keys_role_endpoint_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.api_keys
    ADD CONSTRAINT api_keys_role_endpoint_fkey FOREIGN KEY (role_endpoint) REFERENCES convoy.endpoints(id);


--
-- TOC entry 3467 (class 2606 OID 17228)
-- Name: api_keys api_keys_role_project_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.api_keys
    ADD CONSTRAINT api_keys_role_project_fkey FOREIGN KEY (role_project) REFERENCES convoy.projects(id);


--
-- TOC entry 3468 (class 2606 OID 17223)
-- Name: api_keys api_keys_user_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.api_keys
    ADD CONSTRAINT api_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES convoy.users(id);


--
-- TOC entry 3452 (class 2606 OID 17300)
-- Name: applications applications_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.applications
    ADD CONSTRAINT applications_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3459 (class 2606 OID 17052)
-- Name: devices devices_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.devices
    ADD CONSTRAINT devices_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3447 (class 2606 OID 16956)
-- Name: endpoints endpoints_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.endpoints
    ADD CONSTRAINT endpoints_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3473 (class 2606 OID 17122)
-- Name: event_deliveries event_deliveries_device_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.event_deliveries
    ADD CONSTRAINT event_deliveries_device_id_fkey FOREIGN KEY (device_id) REFERENCES convoy.devices(id);


--
-- TOC entry 3474 (class 2606 OID 17127)
-- Name: event_deliveries event_deliveries_endpoint_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.event_deliveries
    ADD CONSTRAINT event_deliveries_endpoint_id_fkey FOREIGN KEY (endpoint_id) REFERENCES convoy.endpoints(id);


--
-- TOC entry 3475 (class 2606 OID 17261)
-- Name: event_deliveries event_deliveries_event_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.event_deliveries
    ADD CONSTRAINT event_deliveries_event_id_fkey FOREIGN KEY (event_id) REFERENCES convoy.events(id);


--
-- TOC entry 3476 (class 2606 OID 17137)
-- Name: event_deliveries event_deliveries_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.event_deliveries
    ADD CONSTRAINT event_deliveries_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3477 (class 2606 OID 17210)
-- Name: event_deliveries event_deliveries_subscription_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.event_deliveries
    ADD CONSTRAINT event_deliveries_subscription_id_fkey FOREIGN KEY (subscription_id) REFERENCES convoy.subscriptions(id);


--
-- TOC entry 3471 (class 2606 OID 17284)
-- Name: events_endpoints events_endpoints_endpoint_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.events_endpoints
    ADD CONSTRAINT events_endpoints_endpoint_id_fkey FOREIGN KEY (endpoint_id) REFERENCES convoy.endpoints(id) ON DELETE CASCADE;


--
-- TOC entry 3472 (class 2606 OID 17279)
-- Name: events_endpoints events_endpoints_event_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.events_endpoints
    ADD CONSTRAINT events_endpoints_event_id_fkey FOREIGN KEY (event_id) REFERENCES convoy.events(id) ON DELETE CASCADE;


--
-- TOC entry 3469 (class 2606 OID 17246)
-- Name: events events_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.events
    ADD CONSTRAINT events_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3479 (class 2606 OID 17363)
-- Name: events_search events_search_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.events_search
    ADD CONSTRAINT events_search_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3480 (class 2606 OID 17368)
-- Name: events_search events_search_source_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.events_search
    ADD CONSTRAINT events_search_source_id_fkey FOREIGN KEY (source_id) REFERENCES convoy.sources(id);


--
-- TOC entry 3470 (class 2606 OID 17251)
-- Name: events events_source_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.events
    ADD CONSTRAINT events_source_id_fkey FOREIGN KEY (source_id) REFERENCES convoy.sources(id);


--
-- TOC entry 3481 (class 2606 OID 17390)
-- Name: jobs jobs_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.jobs
    ADD CONSTRAINT jobs_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3478 (class 2606 OID 16941)
-- Name: meta_events meta_events_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.meta_events
    ADD CONSTRAINT meta_events_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3453 (class 2606 OID 17008)
-- Name: organisation_invites organisation_invites_organisation_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_invites
    ADD CONSTRAINT organisation_invites_organisation_id_fkey FOREIGN KEY (organisation_id) REFERENCES convoy.organisations(id);


--
-- TOC entry 3454 (class 2606 OID 16966)
-- Name: organisation_invites organisation_invites_role_endpoint_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_invites
    ADD CONSTRAINT organisation_invites_role_endpoint_fkey FOREIGN KEY (role_endpoint) REFERENCES convoy.endpoints(id);


--
-- TOC entry 3455 (class 2606 OID 16901)
-- Name: organisation_invites organisation_invites_role_project_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_invites
    ADD CONSTRAINT organisation_invites_role_project_fkey FOREIGN KEY (role_project) REFERENCES convoy.projects(id);


--
-- TOC entry 3448 (class 2606 OID 17033)
-- Name: organisation_members organisation_members_organisation_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_members
    ADD CONSTRAINT organisation_members_organisation_id_fkey FOREIGN KEY (organisation_id) REFERENCES convoy.organisations(id);


--
-- TOC entry 3449 (class 2606 OID 16961)
-- Name: organisation_members organisation_members_role_endpoint_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_members
    ADD CONSTRAINT organisation_members_role_endpoint_fkey FOREIGN KEY (role_endpoint) REFERENCES convoy.endpoints(id);


--
-- TOC entry 3450 (class 2606 OID 16891)
-- Name: organisation_members organisation_members_role_project_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_members
    ADD CONSTRAINT organisation_members_role_project_fkey FOREIGN KEY (role_project) REFERENCES convoy.projects(id);


--
-- TOC entry 3451 (class 2606 OID 17028)
-- Name: organisation_members organisation_members_user_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisation_members
    ADD CONSTRAINT organisation_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES convoy.users(id);


--
-- TOC entry 3444 (class 2606 OID 16850)
-- Name: organisations organisations_owner_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.organisations
    ADD CONSTRAINT organisations_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES convoy.users(id);


--
-- TOC entry 3457 (class 2606 OID 17102)
-- Name: portal_links_endpoints portal_links_endpoints_endpoint_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.portal_links_endpoints
    ADD CONSTRAINT portal_links_endpoints_endpoint_id_fkey FOREIGN KEY (endpoint_id) REFERENCES convoy.endpoints(id);


--
-- TOC entry 3458 (class 2606 OID 17097)
-- Name: portal_links_endpoints portal_links_endpoints_portal_link_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.portal_links_endpoints
    ADD CONSTRAINT portal_links_endpoints_portal_link_id_fkey FOREIGN KEY (portal_link_id) REFERENCES convoy.portal_links(id);


--
-- TOC entry 3456 (class 2606 OID 17076)
-- Name: portal_links portal_links_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.portal_links
    ADD CONSTRAINT portal_links_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3445 (class 2606 OID 16881)
-- Name: projects projects_organisation_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.projects
    ADD CONSTRAINT projects_organisation_id_fkey FOREIGN KEY (organisation_id) REFERENCES convoy.organisations(id);


--
-- TOC entry 3446 (class 2606 OID 17321)
-- Name: projects projects_project_configuration_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.projects
    ADD CONSTRAINT projects_project_configuration_id_fkey FOREIGN KEY (project_configuration_id) REFERENCES convoy.project_configurations(id);


--
-- TOC entry 3460 (class 2606 OID 17163)
-- Name: sources sources_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.sources
    ADD CONSTRAINT sources_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3461 (class 2606 OID 17334)
-- Name: sources sources_source_verifier_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.sources
    ADD CONSTRAINT sources_source_verifier_id_fkey FOREIGN KEY (source_verifier_id) REFERENCES convoy.source_verifiers(id);


--
-- TOC entry 3462 (class 2606 OID 17195)
-- Name: subscriptions subscriptions_device_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.subscriptions
    ADD CONSTRAINT subscriptions_device_id_fkey FOREIGN KEY (device_id) REFERENCES convoy.devices(id);


--
-- TOC entry 3463 (class 2606 OID 17200)
-- Name: subscriptions subscriptions_endpoint_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.subscriptions
    ADD CONSTRAINT subscriptions_endpoint_id_fkey FOREIGN KEY (endpoint_id) REFERENCES convoy.endpoints(id);


--
-- TOC entry 3464 (class 2606 OID 17190)
-- Name: subscriptions subscriptions_project_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.subscriptions
    ADD CONSTRAINT subscriptions_project_id_fkey FOREIGN KEY (project_id) REFERENCES convoy.projects(id);


--
-- TOC entry 3465 (class 2606 OID 17205)
-- Name: subscriptions subscriptions_source_id_fkey; Type: FK CONSTRAINT; Schema: convoy; Owner: convoy
--

ALTER TABLE ONLY convoy.subscriptions
    ADD CONSTRAINT subscriptions_source_id_fkey FOREIGN KEY (source_id) REFERENCES convoy.sources(id);


-- Completed on 2024-05-27 10:57:03 WAT

--
-- PostgreSQL database dump complete
--

