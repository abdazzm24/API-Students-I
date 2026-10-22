DELETE FROM supervisions
WHERE student_id IN (
    SELECT id
    FROM students
    WHERE nim LIKE 'LOADTEST-M9-%'
);

UPDATE supervisors s
SET supervised_count = (
        SELECT COUNT(*)
        FROM supervisions x
        WHERE x.supervisor_id = s.id
    ),
    quota = 3,
    version = version + 1
WHERE s.name = 'LOADTEST M9';