import '../styles/styles.css';
import { useState, useEffect, useRef } from 'react';
import { WebSocketContext } from '../contexts/WebSocketContext';

function MyApp({ Component, pageProps }) {
  const [data, setData] = useState([]); // Состояние для данных пайплайнов
  const [isLoading, setIsLoading] = useState(true); // Состояние загрузки данных
  const socketRef = useRef(null); // Ссылка на WebSocket-соединение

  // Функция для начальной загрузки данных с API
  const fetchData = async () => {
    try {
      console.log("Fetching initial data from API...");
      const response = await fetch('http://localhost:8080/api/pipelines');
      if (!response.ok) throw new Error('Network response was not ok');
      const result = await response.json();
      console.log("Initial data fetched:", result);
      setData(result || []);
    } catch (error) {
      console.error('Error fetching data:', error);
    } finally {
      setIsLoading(false);
    }
  };

  // Функция для проверки задач на стороне backend
  const checkTasksProgress = async () => {
    try {
      console.log("Checking tasks progress...");
      const response = await fetch('http://localhost:8080/api/check-tasks', { method: 'POST' });
      if (!response.ok) throw new Error('Failed to check tasks progress on backend');
      const result = await response.json();
      console.log("Backend task check result:", result.message);
    } catch (error) {
      console.error("Error checking tasks progress:", error);
    }
  };

  // Функция для обработки входящих сообщений WebSocket
  const handleWebSocketMessage = (event) => {
    try {
      console.log('WebSocket message received:', event.data);
      const receivedData = JSON.parse(event.data);

      if (!receivedData || typeof receivedData !== 'object') {
        console.error('Invalid WebSocket data format:', receivedData);
        return;
      }

      const { action, pipeline_id, task, task_id } = receivedData;
// Обновление состояния данных
      setData((prevData) => {
        return prevData.map((pipeline) => {
          // Если pipeline_id не совпадает, возвращаем текущий pipeline без изменений
          if (pipeline.pipeline_id !== pipeline_id) return pipeline;

          let updatedTasks = pipeline.tasks || [];
// Обработка различных действий
          switch (action) {
            case 'delete_task': // обработка удаления данных задачи и её зависимостей
              updatedTasks = updatedTasks
                .filter((task) => task.task_id !== task_id)
                .map((task) => ({
                  ...task,
                  depends_on: (task.depends_on || []).filter((depId) => depId !== task_id),
                }));
              break;

              case 'update_task': // обработка обновления данных задачи
                if (!task) {
                  console.error("Invalid data for 'update_task':", receivedData);
                  return pipeline;
                }
                updatedTasks = updatedTasks.map((t) =>
                  t.task_id === task.task_id ? { ...t, ...task } : t
                );
                break;
// Добавление новой задачи в список
            case 'add_task':
              if (!task) {
                console.error("Invalid task data for 'add_task':", receivedData);
                return pipeline;
              }
              updatedTasks = [...updatedTasks, task];
              break;

            case 'update_pipeline':
              return { ...pipeline, ...receivedData.pipeline };

            default:
              console.warn('Unknown WebSocket action:', action);
              return pipeline;
          }
// Возвращаем обновленный pipeline с обновленными задачами
          return { ...pipeline, tasks: updatedTasks };
        });
      });
    } catch (error) {
      console.error("Error processing WebSocket data:", error);
    }
  };

  // Установка WebSocket соединения
  const setupWebSocket = () => {
    const socket = new WebSocket('ws://localhost:8080/ws');

    socket.onopen = () => {
      console.log("WebSocket connection opened.");
    };

    socket.onmessage = handleWebSocketMessage;
// Автоматическая попытка переподключения
    socket.onclose = () => {
      console.log("WebSocket connection closed. Attempting to reconnect...");
      setTimeout(setupWebSocket, 1000);
    };

    socketRef.current = socket; // Сохраняем ссылку на WebSocket
  };

  // Подключение WebSocket и загрузка начальных данных
  useEffect(() => {
    fetchData();
    setupWebSocket();

    return () => {
      if (socketRef.current) socketRef.current.close();
    };
  }, []);

  // Добавляем интервал для проверки задач на backend
  useEffect(() => {
    const interval = setInterval(checkTasksProgress, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <WebSocketContext.Provider value={{ data, isLoading, socket: socketRef.current }}>
      <Component {...pageProps} data={data} isLoading={isLoading} />
    </WebSocketContext.Provider>
  );
}

export default MyApp;
