INSERT OR IGNORE INTO prompt_versions
    (id, prompt_version, schema_version, compiler_version, instruction_checksum,
     schema_checksum, evaluation_status, approved_at)
VALUES
    ('prompt_lesson_v3', 'lesson-v3', 'lesson-draft-v1', 'compiler-v3',
     'runtime-verified', 'runtime-verified', 'approved',
     strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
