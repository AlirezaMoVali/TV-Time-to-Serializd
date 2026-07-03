CREATE TYPE export_output_format AS ENUM ('json', 'csv', 'both');

ALTER TABLE user_exports
    ADD COLUMN output_format export_output_format NOT NULL DEFAULT 'json';
