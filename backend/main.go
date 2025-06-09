package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
"strings"
	"github.com/gorilla/mux"
	"github.com/go-yaml/yaml"
	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/lib/pq"
)

// Структура для парсинга YAML
type YamlPipeline struct {
    Pipeline struct {
        Name        string `yaml:"name"`
        Description string `yaml:"description"`
        Tasks       []struct {
            Name        string   `yaml:"name"`
            Description string   `yaml:"description"`
            Status      string   `yaml:"status"`
            Progress    int      `yaml:"progress_percentage"`
            DependsOn   []string `yaml:"depends_on"`
            Assignee    string   `yaml:"assignee"`
			Tags        []string `yaml:"tags"`
        } `yaml:"tasks"`
    } `yaml:"pipeline"`
}

// структура task
type Task struct {
    TaskID      int          `json:"task_id"`
    Name        string       `json:"name"`
    Status      string       `json:"status"`
    Description string       `json:"description"`
    DependsOn   []int        `json:"depends_on"`
    Order       int          `json:"order"`
    StartTime   NullTimeJSON `json:"start_time,omitempty"`
    EndTime     NullTimeJSON `json:"end_time,omitempty"`
    Assignee    string       `json:"assignee,omitempty"`
    Tags        []string     `json:"tags,omitempty"` // Добавьте это поле
}

// Структура ответа для /api/pipeline/{pipeline_id}/tasks/stats
type PipelineTaskStats struct {
    PipelineID    int               `json:"pipeline_id"`
    PipelineName  string            `json:"pipeline_name"`
    TotalTasks    int               `json:"total_tasks"`
    TaskStatuses  map[string]int    `json:"task_statuses"`
    LastUpdated   time.Time         `json:"last_updated"`
}

// Структура ответа для /api/pipelines/average-duration
type AveragePipelineDuration struct {
    TotalPipelinesAnalyzed     int    `json:"total_pipelines_analyzed"`
    AverageDurationSeconds     int64  `json:"average_duration_seconds"`
    AverageDurationHuman       string `json:"average_duration_human_readable"`
    TimePeriod                 struct {
        FromDate string `json:"from_date"`
        ToDate   string `json:"to_date"`
    } `json:"time_period"`
    StatusFilter string `json:"status_filter"`
}


// NullTimeJSON сериализует NullTime в формате JSON
type NullTimeJSON struct {
	sql.NullTime
}

func (nt NullTimeJSON) MarshalJSON() ([]byte, error) {
	if !nt.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(nt.Time)
}

// структура pipeline
type Pipeline struct {
	PipelineID  int          `json:"pipeline_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	StartTime   NullTimeJSON `json:"start_time,omitempty"`
	EndTime     NullTimeJSON `json:"end_time,omitempty"`
	Tasks       []Task       `json:"tasks"`
}

type User struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}

type TaskDetails struct {
	TaskID       int    `json:"task_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	AssignedUser string `json:"assignedUser"`
	PipelineName string `json:"pipelineName"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Duration     string `json:"duration"`
	ErrorCount   int    `json:"error_count"`
	WarningCount int    `json:"warning_count"`
}

var (
	db        *sql.DB
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan interface{})
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clientsMutex = sync.Mutex{}
)

type TaskUpdate struct {
	TaskID int    `json:"task_id"`
	Status string `json:"status"`
}

func initDB() (*sql.DB, error) {
	// Считывает параметры подключения к базе данных из переменных окружения.
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Формирует строку подключения (DSN) для PostgreSQL.
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)
	var err error
	for i := 0; i < 10; i++ {
		db, err = sql.Open("pgx", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Println("Connected to database")
				return db, nil
			}
		}
		log.Printf("Failed to connect to database (attempt %d of 10): %v\n", i+1, err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to database after 10 attempts: %w", err)
}


func uploadPipelineYAMLHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    file, _, err := r.FormFile("yamlFile")
    if err != nil {
        http.Error(w, "Файл не найден", http.StatusBadRequest)
        return
    }
    defer file.Close()

    content, err := ioutil.ReadAll(file)
    if err != nil {
        http.Error(w, "Ошибка чтения файла", http.StatusInternalServerError)
        return
    }

    var yamlData YamlPipeline
    err = yaml.Unmarshal(content, &yamlData)
    if err != nil {
        http.Error(w, "Ошибка парсинга YAML", http.StatusBadRequest)
        return
    }

    var pipelineID int
    err = db.QueryRow(
        `INSERT INTO pipeline (name, description, status) VALUES ($1, $2, 'Pending') RETURNING pipeline_id`,
        yamlData.Pipeline.Name, yamlData.Pipeline.Description,
    ).Scan(&pipelineID)

    if err != nil {
        http.Error(w, "Ошибка создания пайплайна", http.StatusInternalServerError)
        return
    }

    taskNameToID := make(map[string]int)

    currentTime := time.Now()

    for i, t := range yamlData.Pipeline.Tasks {
        var startTime, endTime interface{}
        switch t.Status {
        case "Running":
            startTime = currentTime
            endTime = nil
        case "Completed", "Failed":
            startTime = currentTime
            endTime = currentTime
        case "Pending":
            startTime = nil
            endTime = nil
        default:
            startTime = nil
            endTime = nil
        }

        var assignedTo interface{}
        if t.Assignee != "" {
            var userID int
            err = db.QueryRow(`SELECT user_id FROM "user" WHERE username = $1`, t.Assignee).Scan(&userID)
            if err != nil {
                assignedTo = nil
            } else {
                assignedTo = userID
            }
        } else {
            assignedTo = nil
        }

        // Вставляем задачу с тегами
        // tags в Go []string соответствуют TEXT[] в PostgreSQL
        var taskID int
        err = db.QueryRow(`
            INSERT INTO task (pipeline_id, name, description, status, "order", progress_percentage, assigned_to, start_time, end_time, tags)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING task_id
        `,
            pipelineID, t.Name, t.Description, t.Status, i+1, t.Progress, assignedTo, startTime, endTime, pqStringArray(t.Tags),
        ).Scan(&taskID)

        if err != nil {
            http.Error(w, "Ошибка создания задачи", http.StatusInternalServerError)
            return
        }
        taskNameToID[t.Name] = taskID

        // Если статус Failed, прибавляем +1 к error_count
        errorCount := 0
        if t.Status == "Failed" {
            errorCount = 1
        }

        _, err = db.Exec(`
            INSERT INTO task_metrics (task_id, error_count, warning_count)
            VALUES ($1, $2, 0)
        `, taskID, errorCount)
        if err != nil {
            http.Error(w, "Ошибка при инициализации метрик задачи", http.StatusInternalServerError)
            return
        }
    }

    // Зависимости
    for _, t := range yamlData.Pipeline.Tasks {
        if len(t.DependsOn) > 0 {
            currentTaskID, ok := taskNameToID[t.Name]
            if !ok {
                continue
            }

            for _, depName := range t.DependsOn {
                depTaskID, ok := taskNameToID[depName]
                if !ok {
                    continue
                }

                _, err = db.Exec(`INSERT INTO task_dependency (task_id, depends_on_task_id) VALUES ($1, $2)`, currentTaskID, depTaskID)
                if err != nil {
                    http.Error(w, "Ошибка создания зависимости задач", http.StatusInternalServerError)
                    return
                }
            }
        }
    }

    sendPipelineUpdate(pipelineID)

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":     "Pipeline and tasks successfully created from YAML",
        "pipeline_id": pipelineID,
    })
}

func pqStringArray(arr []string) interface{} {
    if arr == nil {
        return nil
    }
    return pqArray(arr)
}

// вспомогательная функция для приведения []string к формату TEXT[]
func pqArray(a []string) string {
    if len(a) == 0 {
        return "{}"
    }
    escaped := make([]string, len(a))
    for i, v := range a {
        escaped[i] = `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
    }
    return "{" + strings.Join(escaped, ",") + "}"
}

// Добавление тега к задаче
func addTagToTaskHandler(w http.ResponseWriter, r *http.Request) {
    taskIDStr := r.URL.Query().Get("task_id")
    tag := r.URL.Query().Get("tag")

    if taskIDStr == "" || tag == "" {
        http.Error(w, "task_id or tag missing", http.StatusBadRequest)
        return
    }
    taskID, err := strconv.Atoi(taskIDStr)
    if err != nil {
        http.Error(w, "Invalid task_id", http.StatusBadRequest)
        return
    }

    // Добавляем тег, если его нет
    _, err = db.Exec(`
        UPDATE task
        SET tags = array_append(tags, $1)
        WHERE task_id = $2 AND (tags IS NULL OR NOT ($1 = ANY(tags)))
    `, tag, taskID)
    if err != nil {
        http.Error(w, "Ошибка при добавлении тега", http.StatusInternalServerError)
        return
    }

    sendTaskUpdate(taskID)
    w.WriteHeader(http.StatusOK)
    fmt.Fprintln(w, "Тег добавлен к задаче")
}

// Удаление тега у задачи
func removeTagFromTaskHandler(w http.ResponseWriter, r *http.Request) {
    taskIDStr := r.URL.Query().Get("task_id")
    tag := r.URL.Query().Get("tag")

    if taskIDStr == "" || tag == "" {
        http.Error(w, "task_id or tag missing", http.StatusBadRequest)
        return
    }
    taskID, err := strconv.Atoi(taskIDStr)
    if err != nil {
        http.Error(w, "Invalid task_id", http.StatusBadRequest)
        return
    }

    _, err = db.Exec(`
        UPDATE task
        SET tags = array_remove(tags, $1)
        WHERE task_id = $2
    `, tag, taskID)
    if err != nil {
        http.Error(w, "Ошибка при удалении тега", http.StatusInternalServerError)
        return
    }

    sendTaskUpdate(taskID)
    w.WriteHeader(http.StatusOK)
    fmt.Fprintln(w, "Тег удалён у задачи")
}







// Обработчик статистики по задачам
func getPipelineTaskStatsHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    pipelineIDStr := vars["pipeline_id"] // Это из пути /api/pipeline/{pipeline_id}/tasks/stats
    pipelineNameQuery := r.URL.Query().Get("pipeline_name") // Можно передать ?pipeline_name=

    var pipelineID int
    var pipelineName string

    if pipelineNameQuery != "" {
        // Если указано имя пайплайна, то ищем по имени
        err := db.QueryRow(`SELECT pipeline_id, name FROM pipeline WHERE name = $1 LIMIT 1`, pipelineNameQuery).Scan(&pipelineID, &pipelineName)
        if err == sql.ErrNoRows {
            http.Error(w, "Pipeline not found by name", http.StatusNotFound)
            return
        } else if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
    } else {
        // Если имя не указано, используем pipeline_id
        if pipelineIDStr == "" {
            http.Error(w, "Either pipeline_id in the URL or pipeline_name query param must be provided", http.StatusBadRequest)
            return
        }

        pid, err := strconv.Atoi(pipelineIDStr)
        if err != nil {
            http.Error(w, "Invalid pipeline_id", http.StatusBadRequest)
            return
        }
        pipelineID = pid

        err = db.QueryRow(`SELECT name FROM pipeline WHERE pipeline_id = $1`, pipelineID).Scan(&pipelineName)
        if err == sql.ErrNoRows {
            http.Error(w, "Pipeline not found", http.StatusNotFound)
            return
        } else if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
    }

    rows, err := db.Query(`
        SELECT status, COUNT(*) 
        FROM task 
        WHERE pipeline_id = $1
        GROUP BY status
    `, pipelineID)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    taskStatuses := map[string]int{
        "pending":   0,
        "running":   0,
        "completed": 0,
        "failed":    0,
    }

    totalTasks := 0

    for rows.Next() {
        var status string
        var count int
        if err := rows.Scan(&status, &count); err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        totalTasks += count
        switch status {
        case "Pending":
            taskStatuses["pending"] = count
        case "Running":
            taskStatuses["running"] = count
        case "Completed":
            taskStatuses["completed"] = count
        case "Failed":
            taskStatuses["failed"] = count
        }
    }

    stats := PipelineTaskStats{
        PipelineID:   pipelineID,
        PipelineName: pipelineName,
        TotalTasks:   totalTasks,
        TaskStatuses: taskStatuses,
        LastUpdated:  time.Now().UTC(),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}

func getAveragePipelineDurationHandler(w http.ResponseWriter, r *http.Request) {
    statusFilter := r.URL.Query().Get("status")
    if statusFilter == "" {
        statusFilter = "Completed"
    }

    fromDateStr := r.URL.Query().Get("from_date")
    toDateStr := r.URL.Query().Get("to_date")

    var fromDate time.Time
    var toDate time.Time
    var err error

    if fromDateStr == "" {
        fromDate = time.Now().AddDate(0, 0, -7)
    } else {
        fromDate, err = time.Parse("2006-01-02", fromDateStr)
        if err != nil {
            http.Error(w, "Invalid from_date", http.StatusBadRequest)
            return
        }
    }

    if toDateStr == "" {
        toDate = time.Now()
    } else {
        toDate, err = time.Parse("2006-01-02", toDateStr)
        if err != nil {
            http.Error(w, "Invalid to_date", http.StatusBadRequest)
            return
        }
    }

    log.Println("Calculating average pipeline duration with params:")
    log.Println("statusFilter =", statusFilter)
    log.Println("fromDate =", fromDate)
    log.Println("toDate =", toDate)

    row := db.QueryRow(`
        SELECT COUNT(*) as total,
               AVG(EXTRACT(EPOCH FROM (end_time - start_time))) as avg_duration
        FROM pipeline
        WHERE LOWER(status) = LOWER($1)
          AND start_time IS NOT NULL
          AND end_time IS NOT NULL
          AND start_time >= $2
          AND end_time <= $3
    `, statusFilter, fromDate, toDate)

    var totalPipelines int
    var avgSeconds sql.NullFloat64
    err = row.Scan(&totalPipelines, &avgSeconds)
    if err != nil && err != sql.ErrNoRows {
        log.Println("Database error:", err)
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    log.Println("Query result: totalPipelines =", totalPipelines, "avgSeconds =", avgSeconds)

    avgDurationSeconds := int64(0)
    if avgSeconds.Valid {
        avgDurationSeconds = int64(avgSeconds.Float64)
    } else {
        // Если нет ни одного подходящего пайплайна, avgSeconds будет NULL
        // Тогда avgDurationSeconds = 0
        log.Println("No pipelines found for the given criteria.")
    }

    humanReadable := formatDuration(avgDurationSeconds)

    result := AveragePipelineDuration{
        TotalPipelinesAnalyzed: totalPipelines,
        AverageDurationSeconds: avgDurationSeconds,
        AverageDurationHuman:   humanReadable,
        StatusFilter:           statusFilter,
    }
    result.TimePeriod.FromDate = fromDate.Format("2006-01-02")
    result.TimePeriod.ToDate = toDate.Format("2006-01-02")

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}

func formatDuration(seconds int64) string {
    if seconds <= 0 {
        return "0 seconds"
    }
    d := time.Duration(seconds) * time.Second
    hours := int(d.Hours())
    minutes := int(d.Minutes()) % 60
    sec := int(d.Seconds()) % 60

    result := ""
    if hours > 0 {
        result += fmt.Sprintf("%d hour", hours)
        if hours != 1 {
            result += "s"
        }
        if minutes > 0 || sec > 0 {
            result += " "
        }
    }
    if minutes > 0 {
        result += fmt.Sprintf("%d minute", minutes)
        if minutes != 1 {
            result += "s"
        }
        if sec > 0 {
            result += " "
        }
    }
    if sec > 0 {
        result += fmt.Sprintf("%d second", sec)
        if sec != 1 {
            result += "s"
        }
    }

    return result
}


// Добавление нового пайплайна
func createPipelineHandler(w http.ResponseWriter, r *http.Request) {
	// Декодирует данные JSON запроса в структуру Pipeline.
	var pipeline Pipeline
	err := json.NewDecoder(r.Body).Decode(&pipeline)
	if err != nil {
		http.Error(w, "Неверные данные", http.StatusBadRequest)
		return
	}
	// инсертим новый пайплайн в бд
	err = db.QueryRow(
		`INSERT INTO pipeline (name, description, status) VALUES ($1, $2, 'Pending') RETURNING pipeline_id`,
		pipeline.Name, pipeline.Description,
	).Scan(&pipeline.PipelineID)
	if err != nil {
		http.Error(w, "Ошибка создания пайплайна", http.StatusInternalServerError)
		return
	}

	// Устанавливает статус по умолчанию и отправляет данные нового пайплайна клиенту.
	pipeline.Status = "Pending"
	json.NewEncoder(w).Encode(pipeline)
}

// Создание задачи с назначением значения order и зависимости
func createTaskHandler(w http.ResponseWriter, r *http.Request) {
	var task Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {

		http.Error(w, "Неверные данные", http.StatusBadRequest)
		return
	}

	pipelineIDStr := r.URL.Query().Get("pipeline_id")
	pipelineID, err := strconv.Atoi(pipelineIDStr)
	if err != nil {

		http.Error(w, "Некорректный pipeline_id", http.StatusBadRequest)
		return
	}

	// Получаем максимальное значение order для задач в текущем pipeline
	var maxOrder int
	err = db.QueryRow(`SELECT COALESCE(MAX("order"), 0) FROM task WHERE pipeline_id = $1`, pipelineID).Scan(&maxOrder)
	if err != nil {

		http.Error(w, "Ошибка при назначении порядка задачи", http.StatusInternalServerError)
		return
	}

	// Присваиваем order + 1 новой задаче
	newOrder := maxOrder + 1

	// Вставка задачи в таблицу `task` с новым значением order
	err = db.QueryRow(`
    INSERT INTO task (pipeline_id, name, description, status, "order") 
    VALUES ($1, $2, $3, 'Pending', $4) 
    RETURNING task_id
`, pipelineID, task.Name, task.Description, newOrder).Scan(&task.TaskID)
	if err != nil {

		http.Error(w, "Ошибка создания задачи", http.StatusInternalServerError)
		return
	}

	// Инициализация метрик для новой задачи в таблице task_metrics
	_, err = db.Exec(`INSERT INTO task_metrics (task_id, error_count, warning_count) VALUES ($1, 0, 0)`, task.TaskID)
	if err != nil {

		http.Error(w, "Ошибка инициализации метрик для задачи", http.StatusInternalServerError)
		return
	}

	// Устанавливаем зависимость на последнюю задачу, если она существует
	if maxOrder > 0 {
		var lastTaskID int
		err = db.QueryRow(`SELECT task_id FROM task WHERE pipeline_id = $1 AND "order" = $2`, pipelineID, maxOrder).Scan(&lastTaskID)
		if err != nil {

			http.Error(w, "Ошибка при назначении зависимости", http.StatusInternalServerError)
			return
		}

		// Добавляем зависимость в таблицу task_dependency
		_, err = db.Exec(`INSERT INTO task_dependency (task_id, depends_on_task_id) VALUES ($1, $2)`, task.TaskID, lastTaskID)
		if err != nil {

			http.Error(w, "Ошибка при создании зависимости задачи", http.StatusInternalServerError)
			return
		}

		task.DependsOn = []int{lastTaskID}
	} else {
		// Если это первая задача, зависимость отсутствует
		task.DependsOn = []int{}

	}

	task.Status = "Pending"
	task.Order = newOrder

	// Отправка обновленных данных о пайплайне для обновления графа
	sendPipelineUpdate(pipelineID)

	// Ответ клиенту с данными о новой задаче
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(task)

	if err != nil {

		http.Error(w, "Ошибка отправки данных о задаче", http.StatusInternalServerError)
		return
	}

}

// Удаление пайплайна
func deletePipelineHandler(w http.ResponseWriter, r *http.Request) {
	pipelineIDStr := r.URL.Query().Get("pipeline_id")
	pipelineID, err := strconv.Atoi(pipelineIDStr)
	if err != nil {
		http.Error(w, "Некорректный pipeline_id", http.StatusBadRequest)
		return
	}

	_, err = db.Exec(`DELETE FROM pipeline WHERE pipeline_id = $1`, pipelineID)
	if err != nil {
		http.Error(w, "Ошибка удаления пайплайна", http.StatusInternalServerError)
		return
	}

	// Отправка сообщения об удалении пайплайна через WebSocket
	broadcast <- map[string]interface{}{
		"action":      "delete_pipeline",
		"pipeline_id": pipelineID,
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Пайплайн удален")
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskIDStr := r.URL.Query().Get("task_id")
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, "Некорректный task_id", http.StatusBadRequest)
		return
	}

	// Определение pipelineID перед удалением задачи
	var pipelineID int
	err = db.QueryRow(`SELECT pipeline_id FROM task WHERE task_id = $1`, taskID).Scan(&pipelineID)
	if err != nil {

		http.Error(w, "Ошибка при получении pipeline_id задачи", http.StatusInternalServerError)
		return
	}

	// Получаем зависимости удаляемой задачи
	var originalDependsOn []int
	rows, err := db.Query(`SELECT depends_on_task_id FROM task_dependency WHERE task_id = $1`, taskID)
	if err != nil {

		http.Error(w, "Ошибка при получении зависимостей задачи", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var dependsOnID int
		if err := rows.Scan(&dependsOnID); err != nil {

			http.Error(w, "Ошибка при сканировании зависимостей задачи", http.StatusInternalServerError)
			return
		}
		originalDependsOn = append(originalDependsOn, dependsOnID)
	}

	// Получаем зависимые задачи
	var nextTaskIDs []int
	nextRows, err := db.Query(`SELECT task_id FROM task_dependency WHERE depends_on_task_id = $1`, taskID)
	if err != nil {

		http.Error(w, "Ошибка при поиске зависимых задач", http.StatusInternalServerError)
		return
	}
	defer nextRows.Close()

	for nextRows.Next() {
		var nextTaskID int
		if err := nextRows.Scan(&nextTaskID); err != nil {

			http.Error(w, "Ошибка при сканировании зависимых задач", http.StatusInternalServerError)
			return
		}
		nextTaskIDs = append(nextTaskIDs, nextTaskID)
	}

	// Обновление зависимостей для следующих задач
	for _, nextTaskID := range nextTaskIDs {
		_, err = db.Exec(`DELETE FROM task_dependency WHERE task_id = $1 AND depends_on_task_id = $2`, nextTaskID, taskID)
		if err != nil {

			http.Error(w, "Ошибка при удалении зависимости задачи", http.StatusInternalServerError)
			return
		}

		// Назначение оригинальных зависимостей следующей задаче
		for _, originalDepend := range originalDependsOn {
			_, err = db.Exec(`INSERT INTO task_dependency (task_id, depends_on_task_id) VALUES ($1, $2)`, nextTaskID, originalDepend)
			if err != nil {

				http.Error(w, "Ошибка при добавлении зависимости задачи", http.StatusInternalServerError)
				return
			}

		}
	}

	// Удаление зависимостей задачи
	_, err = db.Exec(`DELETE FROM task_dependency WHERE task_id = $1 OR depends_on_task_id = $1`, taskID)
	if err != nil {

		http.Error(w, "Ошибка при удалении зависимостей задачи", http.StatusInternalServerError)
		return
	}

	// Удаление задачи
	_, err = db.Exec(`DELETE FROM task WHERE task_id = $1`, taskID)
	if err != nil {

		http.Error(w, "Ошибка при удалении задачи", http.StatusInternalServerError)
		return
	}

	// Отправка обновления для всех пайплайнов
	sendPipelineUpdate(pipelineID)

	broadcast <- map[string]interface{}{
		"action":      "delete_task",
		"task_id":     taskID,
		"pipeline_id": pipelineID,
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Задача удалена и зависимости обновлены")
}

// Функция обновления данных пайплайна с задачами для WebSocket
// Отправляем обновленный массив задач всего пайплайна
func sendPipelineUpdate(pipelineID int) {
	var pipeline Pipeline

	// получаем информацию по пайпланйам
	err := db.QueryRow(`
        SELECT p.pipeline_id, p.name, p.description, p.status, p.start_time, p.end_time
        FROM pipeline p WHERE p.pipeline_id = $1`, pipelineID).Scan(
		&pipeline.PipelineID, &pipeline.Name, &pipeline.Description, &pipeline.Status, &pipeline.StartTime, &pipeline.EndTime)

	if err != nil {

		return
	}

	//Извлечение задач, их зависимостей и информации о назначенных пользователях
	rows, err := db.Query(`
        SELECT t.task_id, t.name, t.status, t.description, t.start_time, t.end_time, t."order", td.depends_on_task_id,
       u.user_id, r.role_name AS assignee, t.tags
FROM task t
LEFT JOIN task_dependency td ON t.task_id = td.task_id
LEFT JOIN "user" u ON t.assigned_to = u.user_id
LEFT JOIN user_role r ON u.role_id = r.role_id
WHERE t.pipeline_id = $1 ORDER BY t."order" ASC`, pipelineID)
	if err != nil {

		return
	}
	defer rows.Close()

	taskDepends := make(map[int][]int)
	for rows.Next() {
		var task Task
		// данные
		var depID sql.NullInt64     // id задачи
		var userID sql.NullInt64    //пользователя
		var assignee sql.NullString //исполнителя
		var tags pq.StringArray // Используем pq.StringArray для сканирования TEXT[]

		// Чтение данных задачи и её зависимостей из строки результата запроса
		err := rows.Scan(&task.TaskID, &task.Name, &task.Status, &task.Description, &task.StartTime, &task.EndTime, &task.Order, &depID, &userID, &assignee, &tags)
		if err != nil {

			return
		}

		task.Tags = []string(tags) // Конвертируем pq.StringArray в []string

		// Если для задачи указаны зависимости, добавляем их в карту `taskDepends`
		if depID.Valid {
			taskDepends[task.TaskID] = append(taskDepends[task.TaskID], int(depID.Int64))
		}

		if assignee.Valid {
			task.Assignee = assignee.String
		} else {
			task.Assignee = ""
		}

		pipeline.Tasks = append(pipeline.Tasks, task)
	}

	//Присваивание зависимостей задачам пайплайна
	for i, task := range pipeline.Tasks {
		if deps, ok := taskDepends[task.TaskID]; ok {
			pipeline.Tasks[i].DependsOn = deps

		} else {
			pipeline.Tasks[i].DependsOn = []int{}

		}
	}

	broadcast <- map[string]interface{}{
		"action":   "update_pipeline",
		"pipeline": pipeline,
	}

}

// Обработчик для перемещения задач вверх или вниз с обновлением depends_on
func moveTaskHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PipelineID int    `json:"pipelineId"`
		TaskID     int    `json:"taskId"`
		Direction  string `json:"direction"`
	}
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)

		return
	}

	// Получаем текущий порядок задачи
	var currentOrder int
	err = db.QueryRow(`SELECT "order" FROM task WHERE task_id = $1 AND pipeline_id = $2`, request.TaskID, request.PipelineID).Scan(&currentOrder)
	if err != nil {
		http.Error(w, "Task not found", http.StatusInternalServerError)

		return
	}

	// Находим задачу для обмена позиций
	var swapTaskID int
	var newOrder int
	switch request.Direction {
	case "up":
		err = db.QueryRow(`
			SELECT task_id, "order" FROM task 
			WHERE pipeline_id = $1 AND "order" < $2 
			ORDER BY "order" DESC LIMIT 1`, request.PipelineID, currentOrder).Scan(&swapTaskID, &newOrder)
	case "down":
		err = db.QueryRow(`
			SELECT task_id, "order" FROM task 
			WHERE pipeline_id = $1 AND "order" > $2 
			ORDER BY "order" ASC LIMIT 1`, request.PipelineID, currentOrder).Scan(&swapTaskID, &newOrder)
	default:
		http.Error(w, "Invalid move direction", http.StatusBadRequest)

		return
	}

	if err == sql.ErrNoRows {
		http.Error(w, "Cannot move task", http.StatusBadRequest)
		return
	} else if err != nil {
		http.Error(w, "Error moving task", http.StatusInternalServerError)

		return
	}

	// Обмен значениями поля `order`
	_, err = db.Exec(`UPDATE task SET "order" = $1 WHERE task_id = $2`, newOrder, request.TaskID)
	if err != nil {
		http.Error(w, "Error updating task order", http.StatusInternalServerError)

		return
	}

	_, err = db.Exec(`UPDATE task SET "order" = $1 WHERE task_id = $2`, currentOrder, swapTaskID)
	if err != nil {
		http.Error(w, "Error updating task order", http.StatusInternalServerError)

		return
	}

	// Обновляем зависимости
	updateDependencies(request.PipelineID)

	// Отправляем обновленный статус пайплайна через WebSocket
	sendPipelineUpdate(request.PipelineID)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Task order updated and dependencies recalculated")
}

// Функция для пересчёта зависимостей в соответствии с текущим порядком задач в pipeline
func updateDependencies(pipelineID int) {
	// Получаем все задачи в текущем порядке
	rows, err := db.Query(`SELECT task_id, "order" FROM task WHERE pipeline_id = $1 ORDER BY "order" ASC`, pipelineID)
	if err != nil {

		return
	}
	defer rows.Close()

	var tasks []struct {
		TaskID int
		Order  int
	}

	for rows.Next() {
		var task struct {
			TaskID int
			Order  int
		}
		if err := rows.Scan(&task.TaskID, &task.Order); err != nil {

			return
		}
		tasks = append(tasks, task)
	}

	// Обновляем зависимости
	for i, task := range tasks {
		// Очистка текущих зависимостей
		_, err := db.Exec(`DELETE FROM task_dependency WHERE task_id = $1`, task.TaskID)
		if err != nil {

			return
		}

		// Если это не первая задача, назначаем зависимость от предыдущей
		if i > 0 {
			prevTaskID := tasks[i-1].TaskID
			_, err := db.Exec(`INSERT INTO task_dependency (task_id, depends_on_task_id) VALUES ($1, $2)`, task.TaskID, prevTaskID)
			if err != nil {

				return
			}

		}
	}
}

func getTasksByPipeline(pipelineID int) ([]Task, error) {
	// Выполняем sql-запрос для получения задач указанного пайплайна
	rows, err := db.Query(`SELECT task_id, name, status, description, "order" FROM task WHERE pipeline_id = $1 ORDER BY "order"`, pipelineID)

	// если ошибка, то возвращаем пустой список задач - nil
	if err != nil {
		return nil, fmt.Errorf("Ошибка при получении задач для pipeline_id %d: %v", pipelineID, err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		err := rows.Scan(&task.TaskID, &task.Name, &task.Status, &task.Description, &task.Order)
		if err != nil {

			return nil, fmt.Errorf("Ошибка при сканировании задачи для pipeline_id %d: %v", pipelineID, err)
		}

		tasks = append(tasks, task)
	}
	return tasks, nil
}

// Обработчик WebSocket для подключения клиентов
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {

		return
	}
	defer ws.Close()

	// Защищаем доступ к clients
	clientsMutex.Lock()
	clients[ws] = true
	clientsMutex.Unlock()

	for {
		var msg TaskUpdate
		err := ws.ReadJSON(&msg)
		if err != nil {
			// Удаление клиента из мапы при ошибке
			clientsMutex.Lock()
			delete(clients, ws)
			clientsMutex.Unlock()
			break
		}
	}
}

// Рассылка обновлений всем подключенным клиентам
func handleMessages() {
	for {
		msg := <-broadcast

		// Проверяем, что сообщение содержит поле `action`
		if data, ok := msg.(map[string]interface{}); ok {
			if _, exists := data["action"]; !exists {

				continue
			}
		}

		// Рассылаем сообщение всем клиентам
		clientsMutex.Lock()
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {

				client.Close()
				delete(clients, client)
			}
		}
		clientsMutex.Unlock()
	}
}

// Уведомление через WebSocket
func sendUpdatedPipelineToClients() {
	// получаем данные по пайплайну, задачам и зависимостям
	rows, err := db.Query(`
		SELECT p.pipeline_id, p.name, p.status, 
			   t.task_id, t.name, t.status, t.description, td.depends_on_task_id
		FROM pipeline p
		LEFT JOIN task t ON p.pipeline_id = t.pipeline_id
		LEFT JOIN task_dependency td ON t.task_id = td.task_id
		ORDER BY p.pipeline_id ASC, t.task_id ASC
	`)
	if err != nil {
		// Если запрос не удался, просто выходим из функции
		return
	}
	defer rows.Close()

	// хранение данных
	pipelines := make(map[int]*Pipeline)
	taskDepends := make(map[int][]int)

	for rows.Next() {
		var pipelineID, taskID sql.NullInt64
		var depID sql.NullInt64
		var pipelineName, pipelineStatus, taskName, taskStatus, taskDescription sql.NullString

		// Чтение текущей строки результата
		err := rows.Scan(&pipelineID, &pipelineName, &pipelineStatus, &taskID, &taskName, &taskStatus, &taskDescription, &depID)
		if err != nil {

			return
		}

		if pipelines[int(pipelineID.Int64)] == nil {
			pipelines[int(pipelineID.Int64)] = &Pipeline{
				PipelineID: int(pipelineID.Int64),
				Name:       pipelineName.String,
				Status:     pipelineStatus.String,
				Tasks:      []Task{},
			}
		}

		// Если задача существует, добавляем её к пайплайну
		if taskID.Valid {
			task := Task{
				TaskID:      int(taskID.Int64),
				Name:        taskName.String,
				Status:      taskStatus.String,
				Description: taskDescription.String,
				DependsOn:   []int{},
			}
			pipelines[int(pipelineID.Int64)].Tasks = append(pipelines[int(pipelineID.Int64)].Tasks, task)

			if depID.Valid {
				taskDepends[int(taskID.Int64)] = append(taskDepends[int(taskID.Int64)], int(depID.Int64))
			}
		}
	}

	// Присваиваем зависимости задачам
	for _, pipeline := range pipelines {
		for i, task := range pipeline.Tasks {
			if deps, ok := taskDepends[task.TaskID]; ok {
				pipeline.Tasks[i].DependsOn = deps
			}
		}
	}

	var response []Pipeline
	for _, pipeline := range pipelines {
		response = append(response, *pipeline)
	}

	broadcast <- response // отправка обновленной структуры в канал
}

func getPipelinesHandler(w http.ResponseWriter, r *http.Request) {

	// получаем данные о пайплайнах, задачах, зависимостях и исполнителях
	rows, err := db.Query(`
       SELECT p.pipeline_id, p.name, p.description, p.status, p.start_time, p.end_time, 
       t.task_id, t.name, t.status, t.description, t.start_time, t.end_time, t."order",
       u.user_id, r.role_name AS assignee_name, td.depends_on_task_id, t.tags
FROM pipeline p
LEFT JOIN task t ON p.pipeline_id = t.pipeline_id
LEFT JOIN task_dependency td ON t.task_id = td.task_id
LEFT JOIN "user" u ON t.assigned_to = u.user_id
LEFT JOIN user_role r ON u.role_id = r.role_id
ORDER BY p.pipeline_id ASC, t."order" ASC
    `)
	if err != nil {
		// Если запрос завершился ошибкой, возвращаем 500 с описанием проблемы
		http.Error(w, `{"error": "Ошибка получения данных из БД"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	pipelines := make(map[int]*Pipeline)
	taskDepends := make(map[int][]int)

	for rows.Next() {
		// Переменные для чтения данных из строки
		var pipelineID, taskID, assignedTo sql.NullInt64
		var depID sql.NullInt64
		var taskOrder sql.NullInt64
		var pipelineName, pipelineDescription, pipelineStatus, taskName, taskStatus, taskDescription, assigneeRole sql.NullString
		var pipelineStartTime, pipelineEndTime, taskStartTime, taskEndTime sql.NullTime
		var tags pq.StringArray

		// Чтение строки результата
		err := rows.Scan(&pipelineID, &pipelineName, &pipelineDescription, &pipelineStatus, &pipelineStartTime, &pipelineEndTime,
			&taskID, &taskName, &taskStatus, &taskDescription, &taskStartTime, &taskEndTime, &taskOrder,
			&assignedTo, &assigneeRole, &depID, &tags)
		if err != nil {

			http.Error(w, `{"error": "Ошибка обработки данных"}`, http.StatusInternalServerError)
			return
		}

		

		// Если пайплайн ещё не добавлен, создаём его

		if pipelines[int(pipelineID.Int64)] == nil {
			pipelines[int(pipelineID.Int64)] = &Pipeline{
				PipelineID:  int(pipelineID.Int64),
				Name:        pipelineName.String,
				Description: pipelineDescription.String,
				Status:      pipelineStatus.String,
				StartTime:   NullTimeJSON{pipelineStartTime},
				EndTime:     NullTimeJSON{pipelineEndTime},
				Tasks:       []Task{},
			}
		}

		// Если задача существует, добавляем её в список задач пайплайна
		if taskID.Valid {
			task := Task{
				TaskID:      int(taskID.Int64),
				Name:        taskName.String,
				Status:      taskStatus.String,
				Description: taskDescription.String,
				Order:       int(taskOrder.Int64),
				DependsOn:   []int{},
				Assignee:    assigneeRole.String,
				StartTime:   NullTimeJSON{taskStartTime},
				EndTime:     NullTimeJSON{taskEndTime},
				Tags:        []string(tags), // Добавляем теги
			}

			task.Tags = []string(tags)

			pipelines[int(pipelineID.Int64)].Tasks = append(pipelines[int(pipelineID.Int64)].Tasks, task)

			if depID.Valid {
				taskDepends[int(taskID.Int64)] = append(taskDepends[int(taskID.Int64)], int(depID.Int64))
			}
		}
	}

	// Присваиваем зависимости задачам
	for _, pipeline := range pipelines {
		for i, task := range pipeline.Tasks {
			if deps, ok := taskDepends[task.TaskID]; ok {
				pipeline.Tasks[i].DependsOn = deps
			}
		}
	}

	// Формируем окончательный ответ
	var response []Pipeline
	for _, pipeline := range pipelines {
		response = append(response, *pipeline)
	}

	// Отправляем ответ в формате JSON
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {

		http.Error(w, `{"error": "Ошибка формирования ответа"}`, http.StatusInternalServerError)
	}
}

// функция для обработки HTTP-запроса - получения списка пользователей.
func getUsersHandler(w http.ResponseWriter, r *http.Request) {
	// запрос для получения идентификаторов и имен пользователей
	rows, err := db.Query(`SELECT user_id, username FROM "user"`)
	if err != nil {

		http.Error(w, fmt.Sprintf(`{"error": "Ошибка получения данных из БД: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close() // Обеспечиваем закрытие ресурса после завершения функции

	var users []User // Создаем слайс для хранения данных о пользователях
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.UserID, &user.Username); err != nil {

			http.Error(w, `{"error": "Ошибка обработки данных"}`, http.StatusInternalServerError)
			return
		}
		users = append(users, user) // Добавляем пользователя в слайс
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(users); err != nil {

		http.Error(w, `{"error": "Ошибка формирования ответа"}`, http.StatusInternalServerError)
	}
}

// Обработчик для назначения исполнителя к задаче
func assignTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Извлекаем параметры `task_id` и `user_id` из строки запроса
	taskIDStr := r.URL.Query().Get("task_id")
	userIDStr := r.URL.Query().Get("user_id")

	// конвертируем данные в число task_id и user_id
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, "Некорректный task_id", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Некорректный user_id", http.StatusBadRequest)
		return
	}

	// Обновление исполнителя в базе данных
	_, err = db.Exec(`UPDATE task SET assigned_to = $1 WHERE task_id = $2`, userID, taskID)
	if err != nil {
		http.Error(w, "Ошибка назначения исполнителя задачи", http.StatusInternalServerError)
		return
	}

	// Получение обновленных данных задачи, включая исполнителя
	var updatedTask Task
	err = db.QueryRow(`
    SELECT t.task_id, t.name, t.status, t.description, t.start_time, t.end_time, t."order",
           COALESCE(u.username, '') AS assignedUser  -- Обратите внимание на имя поля здесь
    FROM task t
    LEFT JOIN "user" u ON t.assigned_to = u.user_id
    WHERE t.task_id = $1`, taskID).Scan(
		&updatedTask.TaskID,
		&updatedTask.Name,
		&updatedTask.Status,
		&updatedTask.Description,
		&updatedTask.StartTime,
		&updatedTask.EndTime,
		&updatedTask.Order,
		&updatedTask.Assignee,
	)
	if err != nil {

		http.Error(w, "Ошибка при получении обновленных данных задачи", http.StatusInternalServerError)
		return
	}

	// Отправка данных через WebSocket
	broadcast <- map[string]interface{}{
		"action": "update_task",
		"task":   updatedTask,
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Исполнитель задачи успешно назначен")
}

// Функция для обновления статуса задачи и данных связаных с ней (метрики, временные метки)
func updateTaskStatusHandler(w http.ResponseWriter, r *http.Request) {

	// Извлечение параметров `task_id` и `status` из строки запроса
	taskIDStr := r.URL.Query().Get("task_id")
	newStatus := r.URL.Query().Get("status")

	// Проверка наличия обязательных параметров
	if taskIDStr == "" || newStatus == "" {
		http.Error(w, "Task ID or status is missing", http.StatusBadRequest)
		return
	}

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	// Получаем текущий статус задачи для проверки изменений
	var currentStatus string
	err = db.QueryRow(`SELECT status FROM task WHERE task_id = $1`, taskID).Scan(&currentStatus)
	if err != nil {

		http.Error(w, "Error retrieving current task status", http.StatusInternalServerError)
		return
	}

	// Логика для учета ошибок и предупреждений
	if newStatus == "Failed" && currentStatus != "Failed" {

		// Если статуc меняет на failed +1 к ошибкам
		_, err = db.Exec(`
        INSERT INTO task_metrics (task_id, error_count, warning_count)
        VALUES ($1, 1, 0)
        ON CONFLICT (task_id) DO UPDATE SET error_count = task_metrics.error_count + 1
    `, taskID)
		if err != nil {

		}
		// Если статус меняется с запущен на ожидание + 1 к предупреждениям
	} else if newStatus == "Pending" && currentStatus == "Running" {

		_, err = db.Exec(`
        INSERT INTO task_metrics (task_id, error_count, warning_count)
        VALUES ($1, 0, 1)
        ON CONFLICT (task_id) DO UPDATE SET warning_count = task_metrics.warning_count + 1
    `, taskID)
		if err != nil {

		}
	}

	// Обновляем статус задачи и временные метки
	currentTime := time.Now()
	var startTime, endTime interface{}
	switch newStatus {
	case "Running":
		startTime = currentTime
		endTime = nil
	case "Completed", "Failed":
		endTime = currentTime
	case "Pending":
		startTime = nil
		endTime = nil
	}

	query := `
        UPDATE task
        SET status = $1,
            start_time = COALESCE(start_time, $2),
            end_time = $3
        WHERE task_id = $4`

	_, err = db.Exec(query, newStatus, startTime, endTime, taskID)
	if err != nil {

		http.Error(w, "Error updating task status", http.StatusInternalServerError)
		return
	}

	sendTaskUpdate(taskID)

	// Обновляем pipeline
	var pipelineID int
	err = db.QueryRow(`SELECT pipeline_id FROM task WHERE task_id = $1`, taskID).Scan(&pipelineID)
	if err != nil {

		http.Error(w, "Error retrieving pipeline ID", http.StatusInternalServerError)
		return
	}

	sendPipelineUpdate(pipelineID)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Task status updated")
}

// Функция для обновления статуса пайплайна и его временные метки
func updatePipelineStatusHandler(w http.ResponseWriter, r *http.Request) {
	// Извлечение параметров `pipeline_id` и `status` из строки запроса
	pipelineIDStr := r.URL.Query().Get("pipeline_id")
	newStatus := r.URL.Query().Get("status")

	// Проверка наличия обязательных параметров
	if pipelineIDStr == "" || newStatus == "" {
		http.Error(w, "Pipeline ID or status is missing", http.StatusBadRequest)
		return
	}

	// Преобразование `pipeline_id` в число
	pipelineID, err := strconv.Atoi(pipelineIDStr)
	if err != nil {
		http.Error(w, "Invalid pipeline ID", http.StatusBadRequest)
		return
	}

	// Обновление временных меток в зависимости от нового статуса
	currentTime := time.Now()
	var startTime, endTime interface{}

	switch newStatus {
	case "Running":
		startTime = currentTime
		endTime = nil
	case "Completed", "Failed":
		endTime = currentTime
	case "Pending":
		startTime = nil
		endTime = nil
	}

	// Выполнение SQL-запроса для обновления статуса пайплайна
	query := `
        UPDATE pipeline
        SET status = $1,
            start_time = COALESCE(start_time, $2),
            end_time = $3
        WHERE pipeline_id = $4`

	_, err = db.Exec(query, newStatus, startTime, endTime, pipelineID)
	if err != nil {
		http.Error(w, "Failed to update pipeline status", http.StatusInternalServerError)
		return
	}

	// Отправка обновленного состояния пайплайна через WebSocket
	sendPipelineUpdate(pipelineID)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Pipeline status updated")
}

// Проверка и обновление прогресса задач
func checkTasksProgressHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Checking tasks progress...")

	// Обновление задач со статусом 'Pending' -> 'Running'
	_, err := db.Exec(`
        UPDATE task
        SET status = 'Running', start_time = NOW()
        WHERE status = 'Pending' AND progress_percentage > 0
    `)
	if err != nil {
		log.Printf("Error updating tasks from Pending to Running: %v", err)
	}

	// Обновление задач со статусом 'Running' -> 'Failed' // Добавить +1 к ошибкам
	_, err = db.Exec(`
        UPDATE task
        SET status = 'Failed', end_time = NOW()
        WHERE status = 'Running' AND progress_percentage < 100 AND last_updated < NOW() - INTERVAL '10 minutes'
    `)
	if err != nil {
		log.Printf("Error updating tasks from Running to Failed: %v", err)
	}

	// Получение всех задач со статусом 'Running'
	rows, err := db.Query(`SELECT task_id, progress_percentage FROM task WHERE status = 'Running'`)
	if err != nil {
		http.Error(w, "Error retrieving tasks", http.StatusInternalServerError)
		log.Printf("Error retrieving tasks: %v", err)
		return
	}
	defer rows.Close()

	// Добавить с Running -> Pending и добавить +1 к предупреждению

	// Обработка каждой задачи
	for rows.Next() {
		var taskID, progress int
		err := rows.Scan(&taskID, &progress)
		if err != nil {
			log.Printf("Error scanning task: %v", err)
			continue
		}

		// Если прогресс достиг 100%, меняем статус на Completed
		if progress >= 100 {
			_, err = db.Exec(`
                UPDATE task
                SET status = 'Completed', end_time = NOW()
                WHERE task_id = $1`, taskID)
			if err != nil {
				log.Printf("Error updating task %d status to Completed: %v", taskID, err)
				continue
			}

			log.Printf("Task %d marked as Completed due to 100%% progress.", taskID)
			sendTaskUpdateWithProgress(taskID, 100) // Передаем прогресс 100%
		} else {
			// Уведомляем о продолжающихся задачах с их текущим прогрессом
			sendTaskUpdateWithProgress(taskID, progress)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"message": "Task progress updated successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// функция sendTaskUpdateWithProgress для отправки прогресса через WebSocket
func sendTaskUpdateWithProgress(taskID int, progress int) {
	var pipelineID int
	var taskName, taskStatus string
	var startTime, endTime sql.NullTime

	// Получаем pipeline_id, name, status, start_time и end_time
	err := db.QueryRow(`
        SELECT t.pipeline_id, t.name, t.status, t.start_time, t.end_time
        FROM task t
        WHERE t.task_id = $1
    `, taskID).Scan(&pipelineID, &taskName, &taskStatus, &startTime, &endTime)
	if err != nil {
		log.Printf("Ошибка при получении данных задачи task %d: %v", taskID, err)
		return
	}

	// Форматируем дату-время в ISO 8601
	formatTime := func(t sql.NullTime) *string {
		if t.Valid {
			formatted := t.Time.Format("2006-01-02T15:04:05Z07:00") // ISO 8601
			return &formatted
		}
		return nil
	}

	// Отправляем данные через WebSocket
	broadcast <- map[string]interface{}{
		"action": "update_task",
		"task": map[string]interface{}{
			"task_id":    taskID,
			"name":       taskName,
			"status":     taskStatus,
			"progress":   progress,
			"start_time": formatTime(startTime),
			"end_time":   formatTime(endTime),
		},
		"pipeline_id": pipelineID,
	}
}

// Для проверки работоспособность API и подключения к БД.
func handleStatus(w http.ResponseWriter, r *http.Request) {
	// Возвращает сообщение о том, что API работает и подключено к базе данных.
	fmt.Fprintln(w, "API is running and connected to the database!")
}

// форматирование значения в нужный формат
func formatTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	// Форматирует время в строку.
	return t.Time.Format("2006-01-02 15:04:05")
}

// обновление счётчика ошибок и предупреждений, для конкретной задачи
func updateErrorAndWarningCounts(taskID int, isError bool) error {
	query := `UPDATE task_metrics
              SET error_count = CASE WHEN $2 THEN error_count + 1 ELSE error_count END,
                  warning_count = CASE WHEN NOT $2 THEN warning_count + 1 ELSE warning_count END
              WHERE task_id = $1`

	_, err := db.Exec(query, taskID, isError)
	return err
}

// функция для аналитики по пайплайнам
func getPipelineAnalyticsHandler(w http.ResponseWriter, r *http.Request) {

	// Получение фильтров из строки запроса.
	pipelineID := r.URL.Query().Get("pipeline_id")
	statusFilter := r.URL.Query().Get("status")

	// sql-запрос
	query := `
        SELECT
            p.pipeline_id,
            COALESCE(AVG(EXTRACT(EPOCH FROM (t.end_time - t.start_time)) / 60), 0) AS avg_task_execution_time,
            COUNT(*) FILTER (WHERE t.status = 'Failed') AS error_count,
            COUNT(*) FILTER (WHERE t.status = 'Completed')::float / NULLIF(COUNT(*), 0) * 100 AS success_rate,
            COALESCE(AVG(EXTRACT(EPOCH FROM (p.end_time - p.start_time)) / 60), 0) AS avg_pipeline_execution_time,
            p.name AS pipeline_name,
            p.status
        FROM pipeline p
        LEFT JOIN task t ON t.pipeline_id = p.pipeline_id
        WHERE ($1::int IS NULL OR p.pipeline_id = $1::int)
          AND ($2::varchar IS NULL OR p.status = $2::varchar)
        GROUP BY p.pipeline_id, p.name, p.status
    `
	// Выполнение запроса к базе данных
	rows, err := db.Query(query, nilIfEmpty(pipelineID), nilIfEmpty(statusFilter))
	if err != nil {

		http.Error(w, "Ошибка выполнения запроса к базе данных", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Сбор данных аналитики.
	var stats []map[string]interface{}
	for rows.Next() {
		var pipelineID int
		var avgTaskExecutionTime, successRate, avgPipelineExecutionTime float64
		var errorCount int
		var pipelineName, status string

		err := rows.Scan(&pipelineID, &avgTaskExecutionTime, &errorCount, &successRate, &avgPipelineExecutionTime, &pipelineName, &status)
		if err != nil {

			http.Error(w, "Ошибка обработки данных", http.StatusInternalServerError)
			return
		}

		// Добавление данных в массив результатов.
		stats = append(stats, map[string]interface{}{
			"pipeline_id":                 pipelineID,
			"avg_task_execution_time":     avgTaskExecutionTime,
			"error_count":                 errorCount,
			"success_rate":                successRate,
			"avg_pipeline_execution_time": avgPipelineExecutionTime,
			"pipeline_name":               pipelineName,
			"status":                      status,
		})
	}

	if len(stats) == 0 {

	}

	// Возвращаем результат в формате JSON.
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Ошибка при кодировании JSON-ответа: %v", err)
		http.Error(w, "Ошибка формирования ответа", http.StatusInternalServerError)
	}
}

// Помощная функция для обработки пустых значений
func nilIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

// функция для детальной информации о той или иной задаче
func getTaskDetails(w http.ResponseWriter, r *http.Request) {

	// Получение taskId из URL.
	vars := mux.Vars(r)
	taskID, err := strconv.Atoi(vars["taskId"])
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	// Определение переменных для хранения данных задачи.
	var task TaskDetails
	var startTime sql.NullTime
	var endTime sql.NullTime
	var errorCount sql.NullInt32
	var warningCount sql.NullInt32

	// запрос для получения информации о задаче, включая время, метрики и связи.
	query := `
        SELECT 
            t.task_id, 
            t.name, 
            t.description, 
            t.status, 
            COALESCE(u.username, '') AS assigned_user, 
            p.name AS pipeline_name, 
            t.start_time, 
            t.end_time, 
            COALESCE(EXTRACT(EPOCH FROM (t.end_time - t.start_time))::INTEGER, 0) AS duration_seconds,
            COALESCE(tm.error_count, 0),   -- Используем COALESCE для гарантированного значения
            COALESCE(tm.warning_count, 0)  -- Используем COALESCE для гарантированного значения
        FROM 
            task t
        LEFT JOIN 
            "user" u ON t.assigned_to = u.user_id
        LEFT JOIN 
            pipeline p ON t.pipeline_id = p.pipeline_id
        LEFT JOIN 
            task_metrics tm ON t.task_id = tm.task_id
        WHERE 
            t.task_id = $1`

	// Выполнение запроса для одной задачи.
	row := db.QueryRow(query, taskID)

	var durationSeconds int
	// Сканирование результатов в переменные.
	err = row.Scan(
		&task.TaskID,
		&task.Name,
		&task.Description,
		&task.Status,
		&task.AssignedUser,
		&task.PipelineName,
		&startTime,
		&endTime,
		&durationSeconds,
		&errorCount,
		&warningCount,
	)
	if err != nil {

		http.Error(w, "Ошибка загрузки данных", http.StatusInternalServerError)
		return
	}

	// Форматирование данных в нужный вид
	task.StartTime = formatTime(startTime)
	task.EndTime = formatTime(endTime)

	// Форматирование данных в нужный вид
	task.ErrorCount = int(errorCount.Int32)
	task.WarningCount = int(warningCount.Int32)

	// Форматирование данных в нужный вид
	duration := time.Duration(durationSeconds) * time.Second
	task.Duration = fmt.Sprintf("%d дней, %d часов, %d минут",
		int(duration.Hours())/24,
		int(duration.Hours())%24,
		int(duration.Minutes())%60,
	)

	// Отправка данных в формате JSON.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// Функция для отправки обновлений о конкретной задаче
func sendTaskUpdate(taskID int) {
	var pipelineID int
	// Запрос для получения pipeline_id по task_id
	err := db.QueryRow(`SELECT pipeline_id FROM task WHERE task_id = $1`, taskID).Scan(&pipelineID)
	if err != nil {
		log.Printf("Ошибка при получении pipeline_id для task %d: %v", taskID, err)
		return // Завершаем выполнение функции при ошибке
	}

	// Далее идет остальная логика функции
	var task TaskDetails
	var startTime, endTime sql.NullTime
	var errorCount, warningCount sql.NullInt32
	var progressPercentage int

	query := `
        SELECT 
            t.task_id, t.name, t.description, t.status, 
            COALESCE(u.username, '') AS assignedUser,
            p.name AS pipeline_name, t.start_time, t.end_time, 
            COALESCE(EXTRACT(EPOCH FROM (t.end_time - t.start_time))::INTEGER, 0) AS duration_seconds,
            COALESCE(tm.error_count, 0),
            COALESCE(tm.warning_count, 0),
            t.progress_percentage
        FROM task t
        LEFT JOIN "user" u ON t.assigned_to = u.user_id
        LEFT JOIN pipeline p ON t.pipeline_id = p.pipeline_id
        LEFT JOIN task_metrics tm ON t.task_id = tm.task_id
        WHERE t.task_id = $1`

	row := db.QueryRow(query, taskID)
	var durationSeconds int
	err = row.Scan(
		&task.TaskID, &task.Name, &task.Description, &task.Status,
		&task.AssignedUser, &task.PipelineName,
		&startTime, &endTime, &durationSeconds, &errorCount, &warningCount,
		&progressPercentage,
	)
	if err != nil {
		log.Printf("Ошибка при получении данных задачи для task %d: %v", taskID, err)
		return
	}

	task.StartTime = formatTime(startTime)
	task.EndTime = formatTime(endTime)
	task.ErrorCount = int(errorCount.Int32)
	task.WarningCount = int(warningCount.Int32)
	task.Duration = fmt.Sprintf("%d дней, %d часов, %d минут",
		int(durationSeconds/86400),
		int(durationSeconds%86400)/3600,
		int(durationSeconds%3600)/60,
	)

	broadcast <- map[string]interface{}{
		"action": "update_task",
		"task": map[string]interface{}{
			"task_id":             task.TaskID,
			"name":                task.Name,
			"status":              task.Status,
			"description":         task.Description,
			"assignee":            task.AssignedUser,
			"pipeline_name":       task.PipelineName,
			"start_time":          task.StartTime,
			"end_time":            task.EndTime,
			"duration":            task.Duration,
			"error_count":         task.ErrorCount,
			"warning_count":       task.WarningCount,
			"progress_percentage": progressPercentage, // Добавляем progress_percentage
		},
		"pipeline_id": pipelineID, // Передача pipeline_id для фронтенда
	}
}

// Функция для экспорта аналитики по пайплайнам в CSV-файл

func exportAnalyticsToCSVHandler(w http.ResponseWriter, r *http.Request) {
	//запрос для получения аналитики по пайплайнам.
	rows, err := db.Query(`
        SELECT pipeline_id, name, status,
               COALESCE(AVG(EXTRACT(EPOCH FROM (t.end_time - t.start_time)) / 60), 0) AS avg_task_time,
               COALESCE(AVG(EXTRACT(EPOCH FROM (p.end_time - p.start_time)) / 60), 0) AS avg_pipeline_time,
               COUNT(*) FILTER (WHERE t.status = 'Failed') AS error_count,
               COUNT(*) FILTER (WHERE t.status = 'Completed')::float / NULLIF(COUNT(*), 0) * 100 AS success_rate
        FROM pipeline p
        LEFT JOIN task t ON t.pipeline_id = p.pipeline_id
        GROUP BY p.pipeline_id
    `)
	if err != nil {
		http.Error(w, "Ошибка выполнения запроса", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Установка заголовков для формирования CSV-файла.
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=analytics.csv")

	// Инициализация CSV writer.
	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write([]string{"Pipeline ID", "Name", "Status", "Avg Task Time (min)", "Avg Pipeline Time (min)", "Error Count", "Success Rate (%)"})

	// Сканирование строк результата запроса и запись их в CSV.
	for rows.Next() {
		var id int
		var name, status string
		var avgTaskTime, avgPipelineTime float64
		var errorCount int
		var successRate float64

		err := rows.Scan(&id, &name, &status, &avgTaskTime, &avgPipelineTime, &errorCount, &successRate)
		if err != nil {
			http.Error(w, "Ошибка при обработке данных", http.StatusInternalServerError)
			return
		}

		writer.Write([]string{
			strconv.Itoa(id),
			name,
			status,
			strconv.FormatFloat(avgTaskTime, 'f', 2, 64),
			strconv.FormatFloat(avgPipelineTime, 'f', 2, 64),
			strconv.Itoa(errorCount),
			strconv.FormatFloat(successRate, 'f', 2, 64),
		})
	}
}

// разрешаем запросы с других доменов.
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Разрешаем запросы с фронтенда на `localhost:3000`.
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Если метод `OPTIONS`, отправляем OK без обработки.
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Передача запроса следующему обработчику.
		next.ServeHTTP(w, r)
	})
}

// Функция для инициализации сервера, подключения к БД и маршруты.
func main() {
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatalf("Ошибка инициализации базы данных: %v", err)
	}
	defer db.Close()

	// Настройка маршрутов API.
	r := mux.NewRouter()


 // Новые эндпоинты для тегов
    r.HandleFunc("/api/task/add-tag", addTagToTaskHandler).Methods("POST")
    r.HandleFunc("/api/task/remove-tag", removeTagFromTaskHandler).Methods("POST")


	r.HandleFunc("/api/pipeline/{pipeline_id}/tasks/stats", getPipelineTaskStatsHandler).Methods("GET")
	r.HandleFunc("/api/pipelines/average-duration", getAveragePipelineDurationHandler).Methods("GET")
	r.HandleFunc("/api/pipeline/upload-yaml", uploadPipelineYAMLHandler).Methods("POST")
	r.HandleFunc("/api/check-tasks", checkTasksProgressHandler).Methods("POST")
	r.HandleFunc("/api/analytics", getPipelineAnalyticsHandler).Methods("GET")
	r.HandleFunc("/api/users", getUsersHandler).Methods("GET")
	r.HandleFunc("/api/task/assign", assignTaskHandler).Methods("POST")
	r.HandleFunc("/api/task/move", moveTaskHandler).Methods("POST")
	r.HandleFunc("/api/pipeline/delete", deletePipelineHandler).Methods("DELETE")
	r.HandleFunc("/api/task/delete", deleteTaskHandler).Methods("DELETE")
	r.HandleFunc("/status", handleStatus).Methods("GET")
	r.HandleFunc("/api/pipelines", getPipelinesHandler).Methods("GET")
	r.HandleFunc("/api/pipeline/create", createPipelineHandler).Methods("POST")
	r.HandleFunc("/api/task/create", createTaskHandler).Methods("POST")
	r.HandleFunc("/api/task/update", updateTaskStatusHandler).Methods("POST")
	r.HandleFunc("/api/pipeline/update", updatePipelineStatusHandler).Methods("POST")
	r.HandleFunc("/ws", handleConnections)

	// Новый маршрут для получения деталей задачи
	r.HandleFunc("/api/task/{taskId}", getTaskDetails).Methods("GET")

	// Запуск обработчика WebSocket-сообщений
	go handleMessages()

	corsHandler := enableCORS(r)

	// Запуск HTTP-сервера на порту 8080.
	log.Println("Сервер запущен на порту 8080")
	if err := http.ListenAndServe(":8080", corsHandler); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}
