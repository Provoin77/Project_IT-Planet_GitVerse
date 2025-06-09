import React, { useEffect, useState, useRef } from 'react';
import { Bar } from 'react-chartjs-2';
import { Chart as ChartJS, BarElement, CategoryScale, LinearScale, Tooltip, Legend } from 'chart.js';
import jsPDF from 'jspdf';
import 'jspdf-autotable';
import ExcelJS from 'exceljs';
import { saveAs } from 'file-saver';
import Link from 'next/link'; // Добавляем Link для перехода

ChartJS.register(BarElement, CategoryScale, LinearScale, Tooltip, Legend);

const AnalyticsPage = () => {

  // Задаем различные состояния
  const [pipelineStats, setPipelineStats] = useState([]);
  const [selectedStatus, setSelectedStatus] = useState('');
  const [filteredStats, setFilteredStats] = useState([]);
  const [socket, setSocket] = useState(null);
  const [searchText, setSearchText] = useState('');
  const chartRef = useRef(null);

// Максимальное значение для оси Y в графике
  const MAX_Y = 100;

// Функция для форматирования времени из минут в дни, часы и минуты
  const formatTime = (minutes) => {
    const days = Math.floor(minutes / 1440);
    const hours = Math.floor((minutes % 1440) / 60);
    const mins = Math.floor(minutes % 60);
    return { days, hours, mins };
  };

// Функция для получения аналитических данных с сервера
  const fetchAnalyticsData = async (status) => {
    try {
      const url = `http://localhost:8080/api/analytics${status ? `?status=${status}` : ''}`;
      const response = await fetch(url);
      const data = await response.json();
      setPipelineStats(data || []);
    } catch (error) {
      console.error('Ошибка получения аналитики:', error);
      setPipelineStats([]);
    }
  };

 // Хук для установки WebSocket соединения и получения данных
  useEffect(() => {
    // Создание WebSocket соединения
    const createWebSocket = () => {
      const ws = new WebSocket('ws://localhost:8080/ws');

// Обработчик открытия соединения
      ws.onopen = () => {
        console.log('WebSocket connection opened for AnalyticsPage.');
      };

      // Обработчик получения сообщений через WebSocket
      ws.onmessage = (event) => {
        try {
          const receivedData = JSON.parse(event.data);
          if (receivedData.action === 'analytics_update' && receivedData.stats) {
            setPipelineStats(receivedData.stats || []);
          }
        } catch (error) {
          console.error('Ошибка обработки данных WebSocket:', error);
        }
      };

  // Обработчик закрытия соединения с автоматическим повторным подключением
      ws.onclose = () => {
        setTimeout(() => {
          setSocket(createWebSocket());
        }, 1000);
      };

      return ws;
    };

    fetchAnalyticsData(selectedStatus);
    setSocket(createWebSocket());

 // Очистка WebSocket при размонтировании компонента
    return () => {
      if (socket) {
        socket.close();
      }
    };
  }, [selectedStatus]); // Перезапуск при изменении фильтра по статусу


   // Хук для фильтрации статистики по статусу и тексту поиска
  useEffect(() => {
    if (pipelineStats.length > 0) {
      let filtered = selectedStatus
        ? pipelineStats.filter((stat) => stat.status === selectedStatus)
        : pipelineStats;

  // Фильтрация по тексту поиска
      if (searchText) {
        filtered = filtered.filter((stat) =>
          stat.pipeline_name.toLowerCase().includes(searchText.toLowerCase())
        );
      }
      setFilteredStats(filtered);
    } else {
      setFilteredStats([]); // Очищаем данные, если исходная статистика пуста
    }
  }, [pipelineStats, selectedStatus, searchText]);

  // Блок различных вычислений 
  const avgTaskExecutionTime = filteredStats.length
    ? filteredStats.reduce((sum, stat) => sum + (stat.avg_task_execution_time || 0), 0) / filteredStats.length
    : 0;

  const totalErrors = filteredStats.length
    ? filteredStats.reduce((sum, stat) => sum + (stat.error_count || 0), 0)
    : 0;

  const avgPipelineExecutionTime = filteredStats.length
    ? filteredStats.reduce((sum, stat) => sum + (stat.avg_pipeline_execution_time || 0), 0) / filteredStats.length
    : 0;

  const successRate = filteredStats.length
    ? (filteredStats.reduce((sum, stat) => sum + (stat.success_rate || 0), 0) / filteredStats.length) / 100
    : 0;

// Данные для графика
  const chartData = {
    labels: ['Среднее время выполнения задач', 'Количество ошибок', 'Успешность', 'Среднее время выполнения пайплайнов'],
    datasets: [
      {
        label: 'Статистика',
        data: [
          Math.min(avgTaskExecutionTime / 1440, MAX_Y),
          Math.min(totalErrors, MAX_Y),
          Math.min(successRate * 100, MAX_Y),
          Math.min(avgPipelineExecutionTime / 1440, MAX_Y),
        ],
        backgroundColor: [
          'rgba(75, 192, 192, 0.6)',
          'rgba(255, 99, 132, 0.6)',
          'rgba(54, 162, 235, 0.6)',
          'rgba(255, 206, 86, 0.6)',
        ],
        borderColor: [
          'rgba(75, 192, 192, 1)',
          'rgba(255, 99, 132, 1)',
          'rgba(54, 162, 235, 1)',
          'rgba(255, 206, 86, 1)',
        ],
        borderWidth: 1,
      },
    ],
  };

 // Настройки для графика
  const chartOptions = {
    responsive: true,
    maintainAspectRatio: true,
    scales: {
      y: {
        beginAtZero: true,
        max: MAX_Y,
      },
    },
    plugins: {
      tooltip: {
        callbacks: {
          label: (tooltipItem) => {
            const value = tooltipItem.raw;
            const index = tooltipItem.dataIndex;
            if (index === 0 || index === 3) {
              const time = formatTime(value * 1440);
              return `Время: ${time.days} дн., ${time.hours} ч., ${time.mins} мин.`;
            } else if (index === 1) {
              return `Ошибки: ${value}`;
            } else if (index === 2) {
              return `Успешность: ${value}%`;
            }
          },
        },
      },
    },
  };

  const exportToPDF = () => {
    const doc = new jsPDF();
    doc.setFont('helvetica', 'normal');
    doc.text('Pipeline Analytics Report', 10, 10);

    // Очистка текста от недопустимых символов
    const cleanText = (text) => {
        if (text === null || text === undefined) return '';
        return text.toString().replace(/[^\x20-\x7E]/g, ''); // Удаляем любые символы, кроме ASCII
    };

    const dataForPDF = filteredStats.map((stat) => {
        const taskTime = formatTime(stat.avg_task_execution_time);
        const pipelineTime = formatTime(stat.avg_pipeline_execution_time);

        return [
            cleanText(stat.pipeline_id),
            cleanText(stat.pipeline_name),
            cleanText(stat.status),
            cleanText(stat.avg_task_execution_time.toFixed(2) + ' мин.'),
            cleanText(`${taskTime.days} д., ${taskTime.hours} ч., ${taskTime.mins} мин.`),
            cleanText(stat.avg_pipeline_execution_time.toFixed(2) + ' мин.'),
            cleanText(`${pipelineTime.days} д., ${pipelineTime.hours} ч., ${pipelineTime.mins} мин.`),
            cleanText(stat.error_count),
            cleanText(stat.success_rate.toFixed(2) + '%'),
        ];
    });

    // Генерация таблицы с корректировкой стилей
    doc.autoTable({
        head: [
            [
                'Pipeline ID',
                'Pipeline Name',
                'Status',
                'Avg Task Time (min)',
                'Avg Task Time (D:H:M)',
                'Avg Pipeline Time (min)',
                'Avg Pipeline Time (D:H:M)',
                'Error Count',
                'Success Rate (%)',
            ],
        ],
        body: dataForPDF,
        startY: 20,
        margin: { left: 2 }, // Отступ от левого края
        styles: {
            font: 'helvetica',
            fontSize: 10,
            overflow: 'linebreak', // Устранение переполнения
            halign: 'left', // Выравнивание текста по левому краю
            cellPadding: 2,
        },
        columnStyles: {
            0: { cellWidth: 20 },
            1: { cellWidth: 30 },
            2: { cellWidth: 20 },
            3: { cellWidth: 30 },
            4: { cellWidth: 40 },
            5: { cellWidth: 30 },
            6: { cellWidth: 40 },
            7: { cellWidth: 20 },
            8: { cellWidth: 20 },
        },
    });

    // Добавляем график
    const canvas = document.querySelector('canvas');
    const imageData = canvas.toDataURL('image/png');
    doc.addImage(imageData, 'PNG', 10, doc.lastAutoTable.finalY + 20, 190, 100);


    // Сохранение PDF
    doc.save('Pipeline_Analytics_Report.pdf');
};


const exportToExcel = async () => {
  const workbook = new ExcelJS.Workbook();
  const worksheet = workbook.addWorksheet('Pipeline Analytics');

  // Задаем ширину колонок
  worksheet.columns = [
    { header: 'Pipeline ID', key: 'pipeline_id', width: 15 },
    { header: 'Pipeline Name', key: 'pipeline_name', width: 25 },
    { header: 'Status', key: 'status', width: 15 },
    { header: 'Avg Task Time (min)', key: 'avg_task_time', width: 20 },
    { header: 'Avg Task Time (D:H:M)', key: 'avg_task_time_dhm', width: 20 },
    { header: 'Avg Pipeline Time (min)', key: 'avg_pipeline_time', width: 20 },
    { header: 'Avg Pipeline Time (D:H:M)', key: 'avg_pipeline_time_dhm', width: 20 },
    { header: 'Error Count', key: 'error_count', width: 15 },
    { header: 'Success Rate (%)', key: 'success_rate', width: 20 },
  ];

  // Добавляем график в верхней части страницы
  const canvas = chartRef.current.toBase64Image();
  const imageId = workbook.addImage({
    base64: canvas,
    extension: 'png',
  });
  worksheet.addImage(imageId, 'A1:K30'); // Добавляем график

  // Добавляем пустые строки перед заголовком (чтобы заголовок начинался с 32 строки)
  // ДА Да вот на столько топорно, не ну а чё, если она не хочет нормально работать :)
  for (let i = 1; i <= 31; i++) {
    worksheet.getRow(i).values = []; // Добавляем пустую строку
  }

  // Устанавливаем заголовок на 32 строку
  worksheet.getRow(32).values = [
    'Pipeline ID',
    'Pipeline Name',
    'Status',
    'Avg Task Time (min)',
    'Avg Task Time (D:H:M)',
    'Avg Pipeline Time (min)',
    'Avg Pipeline Time (D:H:M)',
    'Error Count',
    'Success Rate (%)',
  ];

  // Начинаем запись данных с 33 строки
  let startRow = 33;

  for (let i = 0; i < filteredStats.length; i++) {
    const stat = filteredStats[i];
    const taskTime = formatTime(stat.avg_task_execution_time);
    const pipelineTime = formatTime(stat.avg_pipeline_execution_time);

    worksheet.getRow(startRow + i).values = [
      stat.pipeline_id,
      stat.pipeline_name,
      stat.status,
      stat.avg_task_execution_time.toFixed(2),
      `${taskTime.days} д., ${taskTime.hours} ч., ${taskTime.mins} мин.`,
      stat.avg_pipeline_execution_time.toFixed(2),
      `${pipelineTime.days} д., ${pipelineTime.hours} ч., ${pipelineTime.mins} мин.`,
      stat.error_count,
      stat.success_rate.toFixed(2),
    ];
  }

  const buffer = await workbook.xlsx.writeBuffer();
  const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
  saveAs(blob, 'Pipeline_Analytics_Report.xlsx');
};



  return (
    <div className="analytics-page">
  <h1 className="analytics-page__title">Аналитика по пайплайнам</h1>
  <div style={{marginBottom: "20px"}}>
        <Link href="/ExtendedAnalyticsPage">
          <button className="analytics-page__button">Расширенная статистика</button>
        </Link>
      </div>
  <div className="analytics-page__filter">
    <label>Поиск по названию пайплайна:</label>
    <input
      type="text"
      value={searchText}
      onChange={(e) => setSearchText(e.target.value)}
      placeholder="Введите название пайплайна"
      className="analytics-page__search"
    />
  </div>
  <div className="analytics-page__filter">
    <label>Фильтр по статусу:</label>
    <select onChange={(e) => setSelectedStatus(e.target.value)} value={selectedStatus} className="analytics-page__select">
      <option value="">Все</option>
      <option value="Pending">Ожидающие</option>
      <option value="Running">Запущенные</option>
      <option value="Completed">Завершенные</option>
      <option value="Failed">Неудачные</option>
    </select>
  </div>
  <div className="analytics-page__buttons">
    <button onClick={exportToPDF} className="analytics-page__button">Экспорт в PDF</button>
    <button onClick={exportToExcel} className="analytics-page__button">Экспорт в Excel</button>
  </div>
  <div className="analytics-page__stats">
    <h2>Текстовая статистика:</h2>
    {filteredStats.length > 0 ? (
      <>
        <p>Среднее время выполнения задач: {`${formatTime(avgTaskExecutionTime).days} дн., ${formatTime(avgTaskExecutionTime).hours} ч., ${formatTime(avgTaskExecutionTime).mins} мин.`}</p>
        <p>Количество ошибок: {totalErrors}</p>
        <p>Успешность выполнения: {(successRate * 100).toFixed(2)}%</p>
        <p>Среднее время выполнения пайплайнов: {`${formatTime(avgPipelineExecutionTime).days} дн., ${formatTime(avgPipelineExecutionTime).hours} ч., ${formatTime(avgPipelineExecutionTime).mins} мин.`}</p>
      </>
    ) : (
      <p>Нет данных для выбранного фильтра.</p>
    )}
  </div>
  <div className="analytics-page__chart">
    <Bar ref={chartRef} data={chartData} options={chartOptions} />
  </div>
</div>
  );
};

export default AnalyticsPage;
