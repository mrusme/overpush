DROP TABLE IF EXISTS applications, targets, users;

CREATE TABLE users (
  id uuid PRIMARY KEY,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  key uuid NOT NULL,
  enable boolean NOT NULL DEFAULT true
);

CREATE TABLE applications (
  id uuid PRIMARY KEY,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  user_id uuid,
  target_id uuid,
  enable boolean NOT NULL DEFAULT false,
  confirmed_at timestamp,
  token uuid NOT NULL,
  name character varying NOT NULL,
  icon_path character varying NOT NULL DEFAULT '',
  format character varying NOT NULL DEFAULT 'custom',
  custom_format jsonb NOT NULL DEFAULT '{}',
  encryption_type character varying NOT NULL DEFAULT 'none',
  encryption_recipients character varying[] NOT NULL DEFAULT '{}',
  encrypt_title boolean NOT NULL DEFAULT false,
  encrypt_message boolean NOT NULL DEFAULT false,
  encrypt_attachment boolean NOT NULL DEFAULT false,
  target_args jsonb NOT NULL DEFAULT '{}',
  latest_input character varying NOT NULL DEFAULT '',
  store_latest_input boolean NOT NULL DEFAULT true,
  stat_received integer NOT NULL DEFAULT 0,
  stat_processed integer NOT NULL DEFAULT 0,
  stat_sent integer NOT NULL DEFAULT 0
);

CREATE TABLE targets (
  id uuid PRIMARY KEY,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  enable boolean NOT NULL DEFAULT false,
  icon_path character varying NOT NULL DEFAULT '',
  name character varying NOT NULL DEFAULT '',
  required_app_args character varying[] NOT NULL DEFAULT '{}',
  type character varying NOT NULL DEFAULT '',
  args jsonb NOT NULL DEFAULT '{}'
);
