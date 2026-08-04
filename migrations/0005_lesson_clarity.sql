INSERT OR IGNORE INTO prompt_versions
    (id, prompt_version, schema_version, compiler_version, instruction_checksum,
     schema_checksum, evaluation_status, approved_at)
VALUES
    ('prompt_lesson_v2', 'lesson-v2', 'lesson-draft-v1', 'compiler-v2',
     'runtime-verified', 'runtime-verified', 'approved',
     strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
