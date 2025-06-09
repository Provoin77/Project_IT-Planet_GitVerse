-- Полная инициализация структуры БД с учетом всех правок и начальных данных

-- Таблица ролей пользователей
CREATE TABLE user_role (
    role_id SERIAL PRIMARY KEY,
    role_name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT
);

-- Таблица пользователей
CREATE TABLE "user" (
    user_id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    role_id INT REFERENCES user_role(role_id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица пайплайнов
CREATE TABLE pipeline (
    pipeline_id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_by INT REFERENCES "user"(user_id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'Pending' CHECK (status IN ('Pending', 'Running', 'Completed', 'Failed')),
    start_time TIMESTAMP,
    end_time TIMESTAMP
);

-- Таблица задач
CREATE TABLE task (
    task_id SERIAL PRIMARY KEY,
    pipeline_id INT REFERENCES pipeline(pipeline_id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    status VARCHAR(20) CHECK (status IN ('Pending', 'Running', 'Completed', 'Failed')),
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    assigned_to INT REFERENCES "user"(user_id) ON DELETE SET NULL,
    "order" INTEGER DEFAULT 0,
    progress_percentage INT DEFAULT 0 CHECK (progress_percentage >= 0 AND progress_percentage <= 100),
    tags TEXT[]
);



-- Таблица зависимостей задач
CREATE TABLE task_dependency (
    task_id INT REFERENCES task(task_id) ON DELETE CASCADE,
    depends_on_task_id INT REFERENCES task(task_id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, depends_on_task_id)
);

-- Таблица логов задач
CREATE TABLE task_log (
    log_id SERIAL PRIMARY KEY,
    task_id INT REFERENCES task(task_id) ON DELETE CASCADE,
    log_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    log_type VARCHAR(20) CHECK (log_type IN ('Info', 'Warning', 'Error')),
    message TEXT,
    error_count INT DEFAULT 0,
    warning_count INT DEFAULT 0
);

-- Таблица для аналитики пайплайнов
CREATE TABLE pipeline_stat (
    stat_id SERIAL PRIMARY KEY,
    pipeline_id INT REFERENCES pipeline(pipeline_id) ON DELETE CASCADE,
    average_duration INTERVAL,
    success_rate DECIMAL(5, 2),
    total_duration INTERVAL,
    stage_success_rate DECIMAL(5, 2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица для хранения метрик выполнения задач
CREATE TABLE task_metrics (
    metric_id SERIAL PRIMARY KEY,
    task_id INT REFERENCES task(task_id) ON DELETE CASCADE UNIQUE,
    execution_duration INTERVAL,
    success_rate DECIMAL(5, 2),
    error_count INT DEFAULT 0,
    warning_count INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица для прав доступа
CREATE TABLE access_control (
    access_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES "user"(user_id) ON DELETE CASCADE,
    pipeline_id INT REFERENCES pipeline(pipeline_id) ON DELETE CASCADE,
    permission_level VARCHAR(20) CHECK (permission_level IN ('Admin', 'Developer', 'Viewer'))
);

-- Индексы для оптимизации запросов
CREATE INDEX idx_pipeline_status ON pipeline(status);
CREATE INDEX idx_task_status ON task(status);
CREATE INDEX idx_task_pipeline ON task(pipeline_id);
CREATE INDEX idx_task_assigned_to ON task(assigned_to);
CREATE INDEX idx_task_log_time ON task_log(log_time);

-- Начальные данные для ролей пользователей
INSERT INTO user_role (role_name, description) VALUES
    ('Admin', 'Администратор'),
    ('Developer', 'Разработчик'),
    ('Viewer', 'Просмотр'),
    ('Tester', 'Отвечает за тестирование задач'),
    ('Manager', 'Управление процессами разработки'),
    ('DevOps', 'Инфраструктура и развертывание приложений'),
    ('Support', 'Техническая поддержка пользователей');

-- Начальные данные для пользователей
INSERT INTO "user" (username, role_id) VALUES
    ('admin_user', 1),         -- Admin
    ('developer_user', 2),     -- Developer
    ('viewer_user', 3),        -- Viewer
    ('tester_user', 4),        -- Tester
    ('manager_user', 5),       -- Manager
    ('devops_user', 6),        -- DevOps
    ('support_user', 7);       -- Support

-- Пример данных для пайплайнов
INSERT INTO pipeline (name, description, created_by, status, start_time, end_time) VALUES
    ('Развертывание веб-приложения', 'Пайплайн для развертывания нового веб-приложения', 1, 'Pending', CURRENT_TIMESTAMP, NULL);

-- Пример данных для задач
INSERT INTO task (pipeline_id, name, description, status, "order") VALUES
    (1, 'Разработка', 'Создание основных функций веб-приложения', 'Pending', 1),
    (1, 'Тестирование', 'Проведение юнит-тестов и интеграционных тестов', 'Pending', 2),
    (1, 'Развертывание', 'Развертывание веб-приложения на сервере в продакшене', 'Pending', 3);

-- Пример данных для зависимостей задач
INSERT INTO task_dependency (task_id, depends_on_task_id) VALUES
    (2, 1),  -- Task 2 depends on Task 1
    (3, 2);  -- Task 3 depends on Task 2




