INSERT INTO supervisors (name, quota)
SELECT 'LOADTEST M9', 3
WHERE NOT EXISTS (
    SELECT 1
    FROM supervisors
    WHERE name = 'LOADTEST M9'
);

INSERT INTO students (nim, name, grade, is_active)
SELECT
    'LOADTEST-M9-' || LPAD(g::text, 2, '0'),
    'Load Test Student ' || g,
    75,
    TRUE
FROM generate_series(1, 30) AS g
ON CONFLICT DO NOTHING;

SELECT id, name
FROM supervisors
WHERE name = 'LOADTEST M9';

SELECT MIN(id) AS student_start, COUNT(*) AS students
FROM students
WHERE nim LIKE 'LOADTEST-M9-%';