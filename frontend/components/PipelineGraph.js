// Импорт необходимых модулей и плагинов
import React, { useEffect, useRef, useState } from 'react';
import cytoscape from 'cytoscape';
import dagre from 'cytoscape-dagre';
import { useRouter } from 'next/router';

// Подключение плагина для компоновки графа
cytoscape.use(dagre);

// Компонент для отображения графа пайплайнов
function PipelineGraph({ pipelines, onDeleteTask }) {
  const cyRef = useRef(null); // Ссылка на Cytoscape instance

  const [positions, setPositions] = useState(() => {
    if (typeof window !== 'undefined') {
      const savedPositions = localStorage.getItem('nodePositions');
      return savedPositions ? JSON.parse(savedPositions) : {};
    }
    return {};
  });

  const [tooltip, setTooltip] = useState({
    visible: false,
    x: 0,
    y: 0,
    taskInfo: null,
  });

  const router = useRouter();

  //  для получения цвета взависимости от статуса задачи
  const getStatusColor = (status) => {
    switch (status) {
      case 'Pending':
        return 'lightgray'; 
      case 'Running':
        return 'blue'; 
      case 'Failed':
        return 'red';
      case 'Completed':
        return 'green'; 
      default:
        return 'lightgray'; // Цвет по умолчанию
    }
  };

  // Сохранение позиций узлов в localStorage
  const savePositionsToLocalStorage = (updatedPositions) => {
    localStorage.setItem('nodePositions', JSON.stringify(updatedPositions));
  };

  // Удаление задачи из графа
  const deleteNode = (taskId) => {
    if (!cyRef.current) return;
    const node = cyRef.current.getElementById(taskId);
    if (node) node.remove();
  };

  // Инициализация Cytoscape и обработка событий
  useEffect(() => {
    if (!cyRef.current) {
      cyRef.current = cytoscape({
        container: document.getElementById('cy'),
        style: [
          {
            // стили для графов
            selector: 'node',
            style: {
              'background-color': 'data(bgColor)', 
              label: 'data(label)',
              'text-valign': 'center',
              color: '#fff',
              'font-size': '12px',
              width: '120px',
              height: '120px',
              'text-wrap': 'wrap',
              'text-max-width': '60px',
              'pie-size': '100%',
              'pie-1-background-color': 'green',
              'pie-2-background-color': 'blue',
              'pie-1-background-size': 'data(pie1Value)',
              'pie-2-background-size': 'data(pie2Value)',
            },
          },
          {
            selector: 'edge',
            style: {
              width: 2,
              'line-color': '#aaa',
              'target-arrow-color': '#aaa',
              'target-arrow-shape': 'triangle',
              'curve-style': 'bezier',
            },
          },
        ],
        layout: { name: 'preset' },
      });

      // Обновление позиций узлов при перетаскивании
      cyRef.current.on('dragfree', 'node', () => {
        const updatedPositions = cyRef.current.nodes().reduce((acc, n) => {
          acc[n.id()] = n.position();
          return acc;
        }, {});
        setPositions(updatedPositions);
        savePositionsToLocalStorage(updatedPositions);
      });

      // Отображение всплывающей подсказки при наведении на узел
      cyRef.current.on('mouseover', 'node', (event) => {
  const node = event.target;
  const taskInfo = node.data('taskInfo');
  const renderedPosition = node.renderedPosition();

  setTooltip({
    visible: true,
    x: renderedPosition.x + 100,
    y: renderedPosition.y - 75,
    taskInfo: {
      ...taskInfo,
      assignee: taskInfo.assignee || 'Unassigned',
      startTime: taskInfo.startTime,
      endTime: taskInfo.endTime,
    },
  });
});

      // Скрытие всплывающей подсказки при уходе курсора с узла
      cyRef.current.on('mouseout', 'node', () => {
        setTooltip((prevTooltip) => ({ ...prevTooltip, visible: false }));
      });

      // Переход на страницу деталей задачи при двойном клике на узел
      cyRef.current.on('dblclick', 'node', (event) => {
        const taskId = event.target.id().replace('task-', '');
        router.push(`/TaskDetails?taskId=${taskId}`);
      });
    }
  }, []);

  // Функция для обновления данных узла
  const updateNodeData = (taskId, updatedTask) => {
    if (!cyRef.current) {
      
      return;
    }
  
    const node = cyRef.current.getElementById(taskId);
  
    if (node && node.isNode()) {
      const progress = updatedTask.progress || 0;
      const status = updatedTask.status || 'Pending';
  
      // Преобразуем дату-время в читаемый формат
      const formatDateTime = (datetime) => {
        if (!datetime) return 'N/A';
        try {
          const date = new Date(datetime);
          if (isNaN(date.getTime())) {
            console.error(`Invalid date received: ${datetime}`);
            return 'Invalid Date';
          }
          return date.toLocaleString();
        } catch (error) {
          console.error(`Error formatting date: ${datetime}`, error);
          return 'Invalid Date';
        }
      };
  
      const startTime = formatDateTime(updatedTask.start_time);
      const endTime = formatDateTime(updatedTask.end_time);
  
      const baseColor = getStatusColor(status);
  
      // Устанавливаем данные узла
      node.data({
        label: `${updatedTask.name} - ${status} (${progress}%)`,
        taskInfo: {
          ...updatedTask,
          progress,
          startTime,
          endTime,
          assignee: updatedTask.assignee || 'Unassigned',
        },
        bgColor: status === 'Running' ? undefined : baseColor,
        pie1Value: status === 'Running' ? progress : 0,
        pie2Value: status === 'Running' ? 100 - progress : 0,
      });
  
      cyRef.current.style().update();
    } else {
      console.warn(`Node with ID ${taskId} not found or is not a valid node.`);
    }
  };
  

  // Функция для обновления графа
// Функция для получения позиции последнего узла в текущем пайплайне
const getLastNodePosition = (pipelineTasks, pipelineYOffset) => {
  let lastPosition = null;

  for (const task of pipelineTasks) {
    const taskId = `task-${task.task_id}`;
    const node = cyRef.current.getElementById(taskId);

    if (node && node.length > 0) {
      lastPosition = node.position(); // Получаем позицию последнего узла
    }
  }

  // Если нет узлов в текущем пайплайне, задаем начальную позицию
  return (
    lastPosition || {
      x: 100, // Начальная x позиция
      y: pipelineYOffset, // Смещение по y для текущего пайплайна
    }
  );
};

const updateGraph = () => {
  if (!pipelines || pipelines.length === 0) {
    cyRef.current.elements().remove();
    return;
  }

  const currentNodeIds = new Set();
  const currentEdgeIds = new Set();
  let pipelineYOffset = 200; // Начальный отступ для первого пайплайна

  for (const pipeline of pipelines) {
    if (!pipeline || !pipeline.tasks || pipeline.tasks.length === 0) {
      pipelineYOffset += 300; // Если пайплайн пустой, увеличиваем общий отступ
      continue;
    }

    // Получаем позицию последнего узла в пайплайне
    let lastNodePosition = getLastNodePosition(pipeline.tasks, pipelineYOffset);

    for (const task of pipeline.tasks) {
      if (!task) continue;

      const taskId = `task-${task.task_id}`;
      const taskPosition = positions[taskId] || {
        x: lastNodePosition.x + 200, // Горизонтальный отступ для новой задачи
        y: lastNodePosition.y + 50, // Вертикальный отступ для новой задачи
      };

      const startTime = task.start_time
        ? new Date(task.start_time).toLocaleString()
        : 'N/A';
      const endTime = task.end_time
        ? new Date(task.end_time).toLocaleString()
        : 'N/A';

      if (cyRef.current.getElementById(taskId).length === 0) {
        // Добавление нового узла
        cyRef.current.add({
          group: 'nodes',
          data: {
            id: taskId,
            label: `${task.name} - ${task.status}`,
            bgColor: getStatusColor(task.status),
            taskInfo: {
              ...task,
              startTime,
              endTime,
              assignee: task.assignee || 'Unassigned',
            },
          },
          position: taskPosition,
        });
      }

      currentNodeIds.add(taskId);

      if (task.depends_on && task.depends_on.length > 0) {
        for (const depId of task.depends_on) {
          const edgeId = `edge-${task.task_id}-${depId}`;
          currentEdgeIds.add(edgeId);
          if (cyRef.current.getElementById(edgeId).length === 0) {
            // Добавление ребра
            cyRef.current.add({
              group: 'edges',
              data: { id: edgeId, source: `task-${depId}`, target: taskId },
            });
          }
        }
      }

      // Обновляем позицию последнего узла
      lastNodePosition = taskPosition;
    }

    pipelineYOffset += 300; // Отступ между пайплайнами
  }

  // Удаление узлов и ребер, которые больше не используются
  cyRef.current.elements().forEach((el) => {
    if (el.isNode() && !currentNodeIds.has(el.id())) el.remove();
    if (el.isEdge() && !currentEdgeIds.has(el.id())) el.remove();
  });

  cyRef.current.layout({ name: 'preset' }).run();
};



  useEffect(() => {
    updateGraph();
  }, [pipelines, positions]);

  // WebSocket для обновления данных
  useEffect(() => {
    const socketListener = (event) => {
      try {
        const receivedData = JSON.parse(event.data);
  
        if (receivedData.action === 'update_task' && receivedData.task) {
          
          const taskId = `task-${receivedData.task.task_id}`;
          updateNodeData(taskId, receivedData.task); // Обновляем данные узла
        }
      } catch (error) {
        console.error('Error processing WebSocket data:', error);
      }
    };
  
    const socket = new WebSocket('ws://localhost:8080/ws');
    socket.onopen = () => {
      console.log('WebSocket connection established.');
    };
    socket.onmessage = socketListener;
  
    socket.onclose = () => {
      console.warn('WebSocket connection closed. Attempting to reconnect...');
      setTimeout(() => {
        const newSocket = new WebSocket('ws://localhost:8080/ws');
        newSocket.addEventListener('message', socketListener);
      }, 1000); // Попытка переподключения через 1 секунду
    };
  
    return () => {
      socket.removeEventListener('message', socketListener);
      socket.close();
    };
  }, []);
  

  return (
    <div style={{ position: 'relative' }}>
      <div
        id="cy"
        style={{ width: '100%', height: '600px', border: '1px solid #ddd', borderRadius: '8px' }}
      />
      {tooltip.visible && (
        <div
          style={{
            position: 'absolute',
            top: tooltip.y,
            left: tooltip.x,
            padding: '8px',
            background: 'rgba(0, 0, 0, 0.7)',
            color: '#fff',
            borderRadius: '4px',
            zIndex: 10,
            maxWidth: '200px',
          }}
        >
          <strong>Task Information:</strong>
          <p>Name: {tooltip.taskInfo.name}</p>
          <p>Status: {tooltip.taskInfo.status}</p>
          <p>Start Time: {tooltip.taskInfo.startTime}</p>
          <p>End Time: {tooltip.taskInfo.endTime}</p>
          <p>Assignee: {tooltip.taskInfo.assignee}</p>
        </div>
      )}
    </div>
  );
}

export default PipelineGraph;
