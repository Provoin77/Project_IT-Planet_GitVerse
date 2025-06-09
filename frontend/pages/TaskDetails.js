import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/router';

// Функция для форматирования времени
const formatTime = (time) => {
  if (!time) return ""; // Если время отсутствует, возвращаем пустую строку
  const date = new Date(time); // Преобразуем время в объект Date
  return `${date.getFullYear()}-${(date.getMonth() + 1).toString().padStart(2, '0')}-${date.getDate().toString().padStart(2, '0')} ${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}:${date.getSeconds().toString().padStart(2, '0')}`;
};

// Функция для форматирования времени из строки в удобный формат "YYYY-MM-DD HH:mm:ss"
function TaskDetails() {
  const router = useRouter(); // Хук для работы с маршрутизацией
  const { taskId } = router.query; // Извлечение ID задачи из URL
  const [taskDetails, setTaskDetails] = useState(null);
  const [socket, setSocket] = useState(null); // Состояние для хранения WebSocket соединения


// Хук для загрузки данных о задаче и настройки WebSocket соединения
  useEffect(() => {
    const createWebSocket = () => {
      const ws = new WebSocket('ws://localhost:8080/ws'); //создание websocket соединения

      ws.onopen = () => {
        console.log("WebSocket connection opened for TaskDetails.");
      };

      ws.onmessage = (event) => {
        try {
          const receivedData = JSON.parse(event.data); // Парсим полученные данные
         
  
          if (receivedData.action === "update_task" && receivedData.task) {
            const task = receivedData.task;
            
   // Проверяем, относится ли обновление к текущей задаче
            if (task.task_id === parseInt(taskId)) {
              setTaskDetails((prevDetails) => ({
                ...prevDetails,
                ...task, // Обновляем информацию о задаче
                // обновление исполнителя, кол-во ошибок и предупреждений
                assignedUser: task.assignee ?? prevDetails.assignedUser,
                error_count: task.error_count,     
                warning_count: task.warning_count,  
              }));
              
            }
          } else {
            
          }
        } catch (error) {
          console.error("Ошибка обработки данных из WebSocket:", error);
        }
      };
  
      ws.onclose = () => {
        // Обработчик закрытия WebSocket соединения
        console.log("WebSocket connection closed for TaskDetails. Reconnecting...");
        setTimeout(() => {
          setSocket(createWebSocket());
        }, 1000);
      };
  
      return ws;
    };

    if (taskId) {
      // Загружаем данные о задаче, если ID задачи существует
      fetch(`http://localhost:8080/api/task/${taskId}`)
        .then((response) => response.json())
        .then((data) => {
          //форматируем данные
          data.start_time = formatTime(data.start_time);
          data.end_time = formatTime(data.end_time);
          setTaskDetails(data); // Сохраняем данные о задаче в состояние
        })
        .catch((error) => console.error("Ошибка загрузки данных о задаче:", error));

      setSocket(createWebSocket());
    }

    return () => {
      // Очистка WebSocket соединения при размонтировании компонента
      if (socket) {
        socket.close();
      }
    };
  }, [taskId]);

  if (!taskDetails) return <p>Загрузка данных...</p>;

  return (
    // Отображение детальной информации о задаче
  <div className="task-details-container">
    <h1>Детальная информация о задаче: {taskDetails.name}</h1>
    <p><strong>Описание:</strong> {taskDetails.description}</p>
    <p className={`task-status ${taskDetails.status.toLowerCase()}`}><strong>Статус:</strong> {taskDetails.status}</p>
    <p><strong>Исполнитель:</strong> {taskDetails.assignedUser}</p>
    <p><strong>Принадлежит пайплайну:</strong> {taskDetails.pipelineName}</p>
    <p><strong>Время начала:</strong> {taskDetails.start_time}</p>
    <p><strong>Время окончания:</strong> {taskDetails.end_time}</p>
    <p><strong>Время выполнения:</strong> {taskDetails.duration}</p>
    <p><strong>Ошибки:</strong> <span className="error-count">{taskDetails.error_count}</span></p>
    <p><strong>Предупреждения:</strong> <span className="warning-count">{taskDetails.warning_count}</span></p>
    <a href="/" className="back-button">Назад к задачам</a>
  </div>
);
}

export default TaskDetails;
