import { useState, useEffect } from 'react';
import Link from 'next/link';
import PipelineGraph from '../components/PipelineGraph';

// показываем уведомление 6 секунд
const NOTIFICATION_TIMEOUT = 6000;

export default function HomePage() {

   // Состояния для пайплайнов, фильтров, поиска и создания новых элементов
  const [pipelines, setPipelines] = useState([]);
  const [selectedStatus, setSelectedStatus] = useState('All');
  const [selectedPipelineStatus, setSelectedPipelineStatus] = useState('All');
  const [searchQuery, setSearchQuery] = useState('');
  const [newPipelineName, setNewPipelineName] = useState('');
  const [newPipelineDescription, setNewPipelineDescription] = useState('');
  const [newTaskName, setNewTaskName] = useState('');
  const [newTaskDescription, setNewTaskDescription] = useState('');
  const [selectedPipelineId, setSelectedPipelineId] = useState(null);
  const [expandedPipelineId, setExpandedPipelineId] = useState(null);
  const [currentPage, setCurrentPage] = useState(1);

  const [selectedTaskId, setSelectedTaskId] = useState(null);
  const [selectedUserId, setSelectedUserId] = useState(null);
  const [filteredUsers, setFilteredUsers] = useState([]);

  // стейт для уведомлений
  const [notification, setNotification] = useState(null);

  const pipelinesPerPage = 5;
  const tasksPerPage = 5;

  const [taskSearchQuery, setTaskSearchQuery] = useState({});
  const [taskCurrentPage, setTaskCurrentPage] = useState({});

  // Новое состояние для фильтрации по тегу
  const [filterTag, setFilterTag] = useState('');

  // Новое состояние для добавления/удаления тегов задач
  const [tagForTask, setTagForTask] = useState('');

  // Функция для фильтрации пайплайнов на основе выбранного статуса
  const filteredPipelines = pipelines
    .filter((pipeline) =>
      (selectedPipelineStatus === 'All' || pipeline.status === selectedPipelineStatus) &&
      (searchQuery === '' || (pipeline.name && pipeline.name.toLowerCase().includes(searchQuery.toLowerCase())))
    );

  const totalPages = Math.ceil(filteredPipelines.length / pipelinesPerPage);

  const paginatedPipelines = filteredPipelines.slice(
    (currentPage - 1) * pipelinesPerPage,
    currentPage * pipelinesPerPage
  );

  useEffect(() => {
    fetchPipelines();
  }, []);

 // Функция для получения списка пайплайнов
  const fetchPipelines = () => {
    fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/pipelines`)
      .then((response) => response.json())
      .then((data) => {
        if (Array.isArray(data)) {
          data.sort((a, b) => a.pipeline_id - b.pipeline_id);
          setPipelines(data.map(pipeline => ({
            ...pipeline,
            tasks: pipeline.tasks || [],
          })));
        } else {
          
        }
      })
      .catch((error) => console.error('Ошибка загрузки данных:', error));
  };

// WebSocket для обновления данных 
  useEffect(() => {
    const ws = new WebSocket(`ws://localhost:8080/ws`);
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (Array.isArray(data)) {
        setPipelines(data); // Если данные - массив, обновляем пайплайны
      } else if (data.status === "Deleted") {
        setPipelines((prevPipelines) =>
          prevPipelines.map((pipeline) => ({
            ...pipeline,
            tasks: pipeline.tasks.filter((task) => task.task_id !== data.task_id),  // Удаляем задачу
          }))
        );
      } else {
        setPipelines((prevPipelines) =>
          prevPipelines.map((pipeline) => ({
            ...pipeline,
            tasks: pipeline.tasks.map((task) =>
              task.task_id === data.task_id ? { ...task, status: data.status } : task // Обновляем статус задачи
            ),
          }))
        );
      }
    };
    return () => ws.close();
  }, []);

  useEffect(() => {
    // Загружаем пользователей
    fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/users`)
      .then((response) => {
        if (!response.ok) {
          throw new Error('Ошибка при загрузке пользователей');
        }
        return response.json();
      })
      .then((data) => setFilteredUsers(data))
      .catch((error) => console.error('Ошибка загрузки пользователей:', error));
  }, []);

  // показ уведомления
  const showNotification = (message) => {
    setNotification(message);
    setTimeout(() => setNotification(null), NOTIFICATION_TIMEOUT);
  };

 // Функция для перемещения задач (вверх или вниз)
  const moveTask = async (pipelineId, taskId, direction) => {
    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/task/move`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ pipelineId, taskId, direction }),
      });
      if (response.ok) fetchPipelines();
    } catch (error) {
      console.error('Ошибка при изменении порядка задач:', error);
    }
  };

// Обновление статуса задачи
  const updateTaskStatus = async (taskId, newStatus, taskName) => {
    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/task/update?task_id=${taskId}&status=${newStatus}`, {
        method: 'POST',
      });
      if (response.ok) {
        fetchPipelines();
        showNotification(`Статус задачи "${taskName}" успешно обновлен на: ${newStatus}`);
      }
    } catch (error) {
      console.error('Ошибка обновления статуса задачи:', error);
      showNotification(`Ошибка обновления статуса задачи "${taskName}"`);
    }
  };

  //обновляем статус пайплайна
  const updatePipelineStatus = async (pipelineId, newStatus, pipelineName) => {
    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/pipeline/update?pipeline_id=${pipelineId}&status=${newStatus}`, {
        method: 'POST',
      });
      if (response.ok) {
        fetchPipelines();
        showNotification(`Статус пайплайна "${pipelineName}" успешно обновлен на: ${newStatus}`);
      }
    } catch (error) {
      console.error('Ошибка обновления статуса пайплайна:', error);
      showNotification(`Ошибка обновления статуса пайплайна "${pipelineName}"`);
    }
  };

  //Удаление пайплайна
  const deletePipeline = async (pipelineId) => {
    try {
      await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/pipeline/delete?pipeline_id=${pipelineId}`, {
        method: 'DELETE',
      });
      setPipelines((prev) => prev.filter((pipeline) => pipeline.pipeline_id !== pipelineId));
    } catch (error) {
      console.error('Ошибка удаления пайплайна:', error);
    }
  };

  // Удалине задач
  const deleteTask = async (taskId, pipelineId) => {
    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/task/delete?task_id=${taskId}`, {
        method: 'DELETE',
      });
      if (response.ok) {
        setPipelines((prevPipelines) =>
          prevPipelines.map((pipeline) => {
            if (pipeline.pipeline_id === pipelineId) {
              const updatedTasks = pipeline.tasks
                .filter((task) => task.task_id !== taskId)
                .map((task) => {
                  if (task.depends_on && task.depends_on.includes(taskId)) {
                    return {
                      ...task,
                      depends_on: task.depends_on.filter((depId) => depId !== taskId),
                    };
                  }
                  return task;
                });
              return { ...pipeline, tasks: updatedTasks };
            }
            return pipeline;
          })
        );
      }
    } catch (error) {
      console.error('Ошибка удаления задачи:', error);
    }
  };

  // создание нового пайплайна
  const createPipeline = async () => {
    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/pipeline/create`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newPipelineName,
          description: newPipelineDescription,
        }),
      });
      const newPipeline = await response.json();
      setPipelines((prev) => [...prev, { ...newPipeline, tasks: [] }]);
      setNewPipelineName('');
      setNewPipelineDescription('');
    } catch (error) {
      console.error('Ошибка создания пайплайна:', error);
    }
  };

  //назначение исполнителя на задачу
  const assignTask = async () => {
    if (!selectedTaskId || !selectedUserId) {
      showNotification("Выберите задачу и исполнителя.");
      return;
    }
    try {
      const response = await fetch(
        `${process.env.NEXT_PUBLIC_API_URL}/api/task/assign?task_id=${selectedTaskId}&user_id=${selectedUserId}`,
        {
          method: "POST",
        }
      );
      if (response.ok) {
        showNotification("Исполнитель успешно назначен.");
        fetchPipelines(); // Обновляем список пайплайнов и задач
        setSelectedTaskId(null); // Сбрасываем выбранную задачу
        setSelectedUserId(null); // Сбрасываем выбранного исполнителя
      } else {
        showNotification("Ошибка назначения исполнителя.");
      }
    } catch (error) {
      console.error("Ошибка при назначении исполнителя:", error);
      showNotification("Ошибка назначения исполнителя.");
    }
  };

  // Создание новой задачи
  const createTask = async (pipelineId) => {
    if (!pipelineId) {
      console.error('Не указан ID пайплайна');
      return;
    }
    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/task/create?pipeline_id=${pipelineId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newTaskName, description: newTaskDescription }),
      });
      const newTask = await response.json();
      setPipelines((prev) =>
        prev.map((pipeline) =>
          pipeline.pipeline_id === pipelineId
            ? { ...pipeline, tasks: [...pipeline.tasks, newTask] }
            : pipeline
        )
      );
      setNewTaskName('');
      setNewTaskDescription('');
    } catch (error) {
      console.error('Ошибка создания задачи:', error);
    }
  };

  const handleStatusFilterChange = (e) => {
    if (!selectedPipelineId) {
      showNotification("Выберите пайплайн для фильтрации по задачам");
    } else {
      setSelectedStatus(e.target.value);
    }
  };
  const handlePipelineStatusFilterChange = (e) => {
    setSelectedPipelineStatus(e.target.value);
  };

  const handleSearchChange = (e) => {
    setSearchQuery(e.target.value);
  };

  const togglePipelineExpansion = (pipelineId) => {
    if (expandedPipelineId === pipelineId) {
      setExpandedPipelineId(null);
      setSelectedPipelineId(null); // Сбросить выбранный пайплайн, если свернут
      setSelectedStatus('All'); // Сбросить фильтр задач на "Все" при закрытии пайплайна
    } else {
      setExpandedPipelineId(pipelineId);
      setSelectedPipelineId(pipelineId); // Установить выбранный пайплайн для отображения только его на графе
    }
  };

  const handlePageChange = (pageNumber) => {
    setCurrentPage(pageNumber);
  };

  const handleTaskSearchChange = (pipelineId, query) => {
    setTaskSearchQuery((prevQueries) => ({
      ...prevQueries,
      [pipelineId]: query
    }));
  };

  const handleTaskPageChange = (pipelineId, pageNumber) => {
    setTaskCurrentPage((prevPages) => ({
      ...prevPages,
      [pipelineId]: pageNumber
    }));
  };

  const handlePipelineSelect = (pipelineId) => {
    setSelectedPipelineId(pipelineId);
  };

  const handleDeleteTaskInGraph = (taskId) => {
    if (cyRef.current) {
    
      cyRef.current.handleDeleteTask(taskId);
    }
  };


  // Функции для добавления/удаления тегов
  const addTagToTask = async () => {
    if (!selectedTaskId || !tagForTask) {
      showNotification("Выберите задачу и введите тег.");
      return;
    }
    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/task/add-tag?task_id=${selectedTaskId}&tag=${encodeURIComponent(tagForTask)}`, {
        method: 'POST'
      });
      if (response.ok) {
        showNotification("Тег добавлен к задаче.");
        fetchPipelines();
      } else {
        showNotification("Ошибка добавления тега.");
      }
    } catch (error) {
      console.error("Ошибка добавления тега:", error);
      showNotification("Ошибка добавления тега.");
    }
  };

  const removeTagFromTask = async () => {
    if (!selectedTaskId || !tagForTask) {
      showNotification("Выберите задачу и введите тег для удаления.");
      return;
    }
    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/task/remove-tag?task_id=${selectedTaskId}&tag=${encodeURIComponent(tagForTask)}`, {
        method: 'POST'
      });
      if (response.ok) {
        showNotification("Тег удалён у задачи.");
        fetchPipelines();
      } else {
        showNotification("Ошибка удаления тега.");
      }
    } catch (error) {
      console.error("Ошибка удаления тега:", error);
      showNotification("Ошибка удаления тега.");
    }
  };


// yaml файлы
  const [yamlFile, setYamlFile] = useState(null);

  const handleYamlFileChange = (e) => {
    setYamlFile(e.target.files[0]);
  };

  const handleYamlUpload = async () => {
    if (!yamlFile) {
      showNotification("Выберите YAML-файл для загрузки.");
      return;
    }

    const formData = new FormData();
    formData.append('yamlFile', yamlFile);

    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/pipeline/upload-yaml`, {
        method: 'POST',
        body: formData
      });

      if (response.ok) {
        showNotification("YAML-файл успешно загружен и пайплайн создан.");
        fetchPipelines(); // Обновляем список пайплайнов
      } else {
        showNotification("Ошибка загрузки YAML-файла.");
      }
    } catch (error) {
      console.error("Ошибка загрузки YAML-файла:", error);
      showNotification("Ошибка загрузки YAML-файла.");
    }
  };


  return (
    <div className="container">
      <h1>CI/CD Пайплайны</h1>

      <Link href="/AnalyticsPage">
  <button>Статистика</button>
</Link>

<div style={{ marginTop: '10px', marginBottom: '20px' }}>
        <input type="file" accept=".yaml,.yml" onChange={handleYamlFileChange} />
        <button onClick={handleYamlUpload}>Загрузить YAML</button>
      </div>


      {/* Уведомление */}
      {notification && <div className="notification">{notification}</div>}

      {/* Фильтр по статусу пайплайнов */}
      <div className="status-filter">
        <label>Фильтр по статусу пайплайнов: </label>
        <select value={selectedPipelineStatus} onChange={handlePipelineStatusFilterChange}>
          <option value="All">Все</option>
          <option value="Pending">Ожидание</option>
          <option value="Running">Запуск</option>
          <option value="Completed">Завершено</option>
          <option value="Failed">Остановлено</option>
        </select>
      </div>

      {/* Фильтр по статусу задач */}
      <div className="status-filter">
        <label>Фильтр по статусу задач: </label>
        <select value={selectedStatus} onChange={handleStatusFilterChange}>
          <option value="All">Все</option>
          <option value="Pending">Ожидание</option>
          <option value="Running">Запуск</option>
          <option value="Completed">Завершено</option>
          <option value="Failed">Остановлено</option>
        </select>
      </div>

      {/* Поле для поиска по названию пайплайнов */}
      <div className="search-bar">
        <input
          type="text"
          placeholder="Поиск по названию пайплайна..."
          value={searchQuery}
          onChange={handleSearchChange}
        />
      </div>

      {/* Форма создания нового пайплайна */}
      <div className="create-section">
        <h3>Создать новый пайплайн</h3>
        <input
          type="text"
          placeholder="Название пайплайна"
          value={newPipelineName}
          onChange={(e) => setNewPipelineName(e.target.value)}
        />
        <input
          type="text"
          placeholder="Описание"
          value={newPipelineDescription}
          onChange={(e) => setNewPipelineDescription(e.target.value)}
        />
        <button onClick={createPipeline}>Создать пайплайн</button>
      </div>

         {/* Добавим секцию управления тегами задач */}
         <div>
        <h3>Управление тегами задачи</h3>
        <select onChange={(e) => setSelectedTaskId(e.target.value)} value={selectedTaskId || ""}>
          <option value="">Выберите задачу</option>
          {/* Покажем все задачи всех отображённых пайплайнов, или только если раскрыт пайплайн */}
          {paginatedPipelines.flatMap(p => p.tasks).map(task => (
            <option key={task.task_id} value={task.task_id}>{task.name}</option>
          ))}
        </select>
        <input
          type="text"
          placeholder="Тег"
          value={tagForTask}
          onChange={(e) => setTagForTask(e.target.value)}
        />
        <button onClick={addTagToTask}>Добавить тег</button>
        <button onClick={removeTagFromTask}>Удалить тег</button>
      </div>

      {/* Добавим фильтр по тегу для отображения на графе */}
      <div className="search-bar">
        <input
          type="text"
          placeholder="Фильтр по тегу задач..."
          value={filterTag}
          onChange={(e) => setFilterTag(e.target.value)}
        />
      </div>

      {/* Отображение пайплайнов с пагинацией */}
      {paginatedPipelines.map((pipeline) => {
        const tasksForCurrentPage = (pipeline.tasks || [])
          .filter((task) => task && (selectedStatus === 'All' || task.status === selectedStatus))
          .filter((task) => {
            const query = taskSearchQuery[pipeline.pipeline_id]?.toLowerCase() || '';
            return task && (query === '' || (task.name && task.name.toLowerCase().includes(query)));
          })
          .slice(
            ((taskCurrentPage[pipeline.pipeline_id] || 1) - 1) * tasksPerPage,
            (taskCurrentPage[pipeline.pipeline_id] || 1) * tasksPerPage
          );
          
        return (
          <div key={pipeline.pipeline_id} className="pipeline">
           <h2 onClick={() => togglePipelineExpansion(pipeline.pipeline_id)}>
  {pipeline.name} - <span className={`status-${pipeline.status?.toLowerCase() || 'unknown'}`}>{pipeline.status || 'Unknown'}</span>
</h2>
            {expandedPipelineId === pipeline.pipeline_id && (
              <div>
                <p>{pipeline.description}</p>

                {/* Кнопки управления пайплайном */}
                <button onClick={() => updatePipelineStatus(pipeline.pipeline_id, 'Running', pipeline.name)}>Запуск</button>
                <button onClick={() => updatePipelineStatus(pipeline.pipeline_id, 'Pending', pipeline.name)}>Ожидание</button>
                <button onClick={() => updatePipelineStatus(pipeline.pipeline_id, 'Completed', pipeline.name)}>Завершить</button>
                <button onClick={() => updatePipelineStatus(pipeline.pipeline_id, 'Failed', pipeline.name)}>Остановить</button>
                <button onClick={() => deletePipeline(pipeline.pipeline_id)}>Удалить пайплайн</button>

                {/* Блок выбора задачи и исполнителя */}
                <div className="assign-task-section">
                  <h4>Назначить исполнителя задаче</h4>
                  <div className="assign-task">
                    <select
                      onChange={(e) => setSelectedTaskId(e.target.value)}
                      value={selectedTaskId || ""}
                    >
                      <option value="">Выберите задачу</option>
                      {tasksForCurrentPage.slice(0, 5).map(task => (
                        <option key={task.task_id} value={task.task_id}>{task.name}</option>
                      ))}
                    </select>

                    <select
                      onChange={(e) => setSelectedUserId(e.target.value)}
                      value={selectedUserId || ""}
                    >
                      <option value="">Выберите исполнителя</option>
                      {filteredUsers.map(user => (
                        <option key={user.user_id} value={user.user_id}>{user.username}</option>
                      ))}
                    </select>

                    <button onClick={assignTask}>Добавить исполнителя</button>
                  </div>
                </div>

                {/* Блок добавления новой задачи */}
                <div className="create-task-section">
                  <h4>Добавить новую задачу в пайплайн</h4>
                  <input
                    type="text"
                    placeholder="Название задачи"
                    value={newTaskName}
                    onChange={(e) => setNewTaskName(e.target.value)}
                  />
                  <input
                    type="text"
                    placeholder="Описание задачи"
                    value={newTaskDescription}
                    onChange={(e) => setNewTaskDescription(e.target.value)}
                  />
                  <button onClick={() => createTask(pipeline.pipeline_id)}>Добавить задачу</button>
                </div>

                {/* Фильтр задач внутри пайплайна */}
                <div className="search-bar">
                  <input
                    type="text"
                    placeholder="Поиск по задачам..."
                    value={taskSearchQuery[pipeline.pipeline_id] || ''}
                    onChange={(e) => handleTaskSearchChange(pipeline.pipeline_id, e.target.value)}
                  />
                </div>

                {/* Отображение задач с пагинацией */}
                <ul>
                  {tasksForCurrentPage.map((task, index) => (
                    <li key={task.task_id} className="task-item">
                      <Link href={`/tasks/${task.task_id}`}>
                        <a>
                        <span>{task.name}</span> - <span className={`status-${task.status?.toLowerCase() || 'unknown'}`}>{task.status || 'Unknown'}</span>
                        </a>
                      </Link>
                      <button onClick={() => updateTaskStatus(task.task_id, 'Running', task.name)}>Запуск</button>
                      <button onClick={() => updateTaskStatus(task.task_id, 'Pending', task.name)}>Ожидание</button>
                      <button onClick={() => updateTaskStatus(task.task_id, 'Completed', task.name)}>Завершить</button>
                      <button onClick={() => updateTaskStatus(task.task_id, 'Failed', task.name)}>Остановить</button>
                      <button onClick={() => deleteTask(task.task_id, pipeline.pipeline_id)}>Удалить задачу</button>
                      <button onClick={() => moveTask(pipeline.pipeline_id, task.task_id, 'up')} disabled={index === 0}>↑</button>
                      <button onClick={() => moveTask(pipeline.pipeline_id, task.task_id, 'down')} disabled={index === pipeline.tasks.length - 1}>↓</button>
                    </li>
                  ))}
                </ul>

                {/* Пагинация задач */}
                <div className="task-pagination">
                  {Array.from(
                    { length: Math.ceil(pipeline.tasks.filter((task) => selectedStatus === 'All' || task.status === selectedStatus)
                      .filter((task) => {
                        const query = taskSearchQuery[pipeline.pipeline_id]?.toLowerCase() || '';
                        return query === '' || task.name.toLowerCase().includes(query);
                      }).length / tasksPerPage)
                    },
                    (_, index) => (
                      <button
                        key={index}
                        onClick={() => handleTaskPageChange(pipeline.pipeline_id, index + 1)}
                        className={index + 1 === (taskCurrentPage[pipeline.pipeline_id] || 1) ? 'active' : ''}
                      >
                        {index + 1}
                      </button>
                    )
                  )}
                </div>
              </div>
            )}
          </div>
        );
      })}

      {/* Пагинация пайплайнов */}
      <div className="pagination">
        {Array.from({ length: totalPages }, (_, index) => (
          <button
            key={index}
            onClick={() => handlePageChange(index + 1)}
            className={index + 1 === currentPage ? 'active' : ''}
          >
            {index + 1}
          </button>
        ))}
      </div>

      {/* Блок визуализации пайплайнов */}
<h2>Визуализация Пайплайнов</h2>


<PipelineGraph  
  pipelines={
    selectedPipelineId 
      ? (() => {
          const pipeline = filteredPipelines.find(p => p.pipeline_id === selectedPipelineId);
          if (!pipeline) return [];

          // Применяем все фильтры к задачам
          const filteredTasks = pipeline.tasks
            .filter(task => selectedStatus === 'All' || task.status === selectedStatus)
            .filter((task) => {
              const query = taskSearchQuery[pipeline.pipeline_id]?.toLowerCase() || '';
              return query === '' || (task.name && task.name.toLowerCase().includes(query));
            })
            .filter(task => filterTag === ''
              || (task.tags && task.tags.some(t => t.toLowerCase().includes(filterTag.toLowerCase()))));

          // Создаём множество ID отфильтрованных задач
          const filteredTaskIDs = new Set(filteredTasks.map(t => t.task_id));

          // Удаляем зависимости на задачи, которых больше нет
          const finalTasks = filteredTasks.map(t => ({
            ...t,
            depends_on: t.depends_on.filter(depId => filteredTaskIDs.has(depId))
          }));

          return [{
            ...pipeline,
            tasks: finalTasks
          }];
        })()
      : paginatedPipelines.map(pipeline => {
          const filteredTasks = pipeline.tasks
            .filter(task => selectedStatus === 'All' || task.status === selectedStatus)
            .filter((task) => {
              const query = taskSearchQuery[pipeline.pipeline_id]?.toLowerCase() || '';
              return query === '' || (task.name && task.name.toLowerCase().includes(query));
            })
            .filter(task => filterTag === ''
              || (task.tags && task.tags.some(t => t.toLowerCase().includes(filterTag.toLowerCase()))));

          // Создаём множество ID отфильтрованных задач
          const filteredTaskIDs = new Set(filteredTasks.map(t => t.task_id));

          // Удаляем зависимости на задачи, которых нет после фильтрации
          const finalTasks = filteredTasks.map(t => ({
            ...t,
            depends_on: t.depends_on.filter(depId => filteredTaskIDs.has(depId))
          }));

          return {
            ...pipeline,
            tasks: finalTasks
          };
        })
  }
  onDeleteTask={handleDeleteTaskInGraph}
/>


    </div>
  );
}
