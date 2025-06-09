import React, { useState, useEffect } from 'react';

export default function ExtendedAnalyticsPage() {
  const [pipelineId, setPipelineId] = useState('');
  const [pipelineName, setPipelineName] = useState('');
  const [pipelineStats, setPipelineStats] = useState(null);
  const [avgDurationData, setAvgDurationData] = useState(null);

  const [statusFilter, setStatusFilter] = useState('completed');
  const [fromDate, setFromDate] = useState('2024-12-01');
  const [toDate, setToDate] = useState('2024-12-31');


  const fetchPipelineStats = async () => {
    let url = '';
    // Если указано имя, то используем имя, иначе pipeline_id
    if (pipelineName.trim() !== '') {
      url = `${process.env.NEXT_PUBLIC_API_URL}/api/pipeline/0/tasks/stats?pipeline_name=${encodeURIComponent(pipelineName)}`;
    } else if (pipelineId.trim() !== '') {
      url = `${process.env.NEXT_PUBLIC_API_URL}/api/pipeline/${pipelineId}/tasks/stats`;
    } else {
      alert("Укажите pipeline_id или pipeline_name");
      return;
    }

    try {
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error('Ошибка при получении статистики задач пайплайна');
      }
      const data = await response.json();
      setPipelineStats(data);
    } catch (error) {
      console.error(error);
      alert("Не удалось получить статистику. Проверьте логи консоли или параметры запроса.");
    }
  };

  const fetchAverageDuration = async () => {
    let url = `${process.env.NEXT_PUBLIC_API_URL}/api/pipelines/average-duration?status=${statusFilter}&from_date=${fromDate}&to_date=${toDate}`;

    try {
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error('Ошибка при получении среднего времени выполнения');
      }
      const data = await response.json();
      setAvgDurationData(data);
    } catch (error) {
      console.error(error);
      alert("Не удалось получить среднее время выполнения. Проверьте логи или параметры.");
    }
  };

  useEffect(() => {
    // Можно по умолчанию загрузить данные для какого-нибудь pipeline_id, если нужно.
  }, []);

   return (
    <div style={{padding: '20px', fontFamily: 'Arial, sans-serif'}}>
      <h1>Расширенная статистика</h1>
      
      <div style={{marginBottom: '20px'}}>
        <h2>Статистика по задачам пайплайна</h2>
        <div style={{marginBottom:'10px'}}>
          <input
            type="text"
            placeholder="Введите pipeline_id"
            value={pipelineId}
            onChange={(e) => setPipelineId(e.target.value)}
            style={{padding:'5px', marginRight:'10px'}}
          />
          <input
            type="text"
            placeholder="Введите pipeline_name"
            value={pipelineName}
            onChange={(e) => setPipelineName(e.target.value)}
            style={{padding:'5px', marginRight:'10px'}}
          />
        </div>
        
        <button onClick={fetchPipelineStats} style={{padding:'5px 10px'}}>Загрузить статистику по задачам</button>

        {pipelineStats && (
          <div style={{marginTop:'20px', background:'#f4f6f8', padding:'10px', borderRadius:'8px'}}>
            <h3>Pipeline ID: {pipelineStats.pipeline_id}</h3>
            <p>Название: {pipelineStats.pipeline_name}</p>
            <p>Всего задач: {pipelineStats.total_tasks}</p>
            <p>Статусы задач:</p>
            <ul>
              <li>Pending: {pipelineStats.task_statuses.pending}</li>
              <li>Running: {pipelineStats.task_statuses.running}</li>
              <li>Completed: {pipelineStats.task_statuses.completed}</li>
              <li>Failed: {pipelineStats.task_statuses.failed}</li>
            </ul>
            <p>Последнее обновление: {pipelineStats.last_updated}</p>
          </div>
        )}
      </div>

      <div style={{marginBottom: '20px'}}>
        <h2>Среднее время выполнения пайплайнов</h2>
        <label>Статус: </label>
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} style={{marginRight:'10px', padding:'5px'}}>
          <option value="completed">completed</option>
          <option value="failed">failed</option>
        </select>

        <label>From Date (YYYY-MM-DD): </label>
        <input
          type="text"
          value={fromDate}
          onChange={(e) => setFromDate(e.target.value)}
          style={{padding:'5px', marginRight:'10px'}}
        />
        <label>To Date (YYYY-MM-DD): </label>
        <input
          type="text"
          value={toDate}
          onChange={(e) => setToDate(e.target.value)}
          style={{padding:'5px', marginRight:'10px'}}
        />

        <button onClick={fetchAverageDuration} style={{padding:'5px 10px'}}>Загрузить среднее время</button>

        {avgDurationData && (
          <div style={{marginTop:'20px', background:'#f4f6f8', padding:'10px', borderRadius:'8px'}}>
            <p>Всего анализируемых пайплайнов: {avgDurationData.total_pipelines_analyzed}</p>
            <p>Средняя длительность (секунды): {avgDurationData.average_duration_seconds}</p>
            <p>Человекочитаемый формат: {avgDurationData.average_duration_human_readable}</p>
            <p>Период: {avgDurationData.time_period.from_date} до {avgDurationData.time_period.to_date}</p>
            <p>Фильтр по статусу: {avgDurationData.status_filter}</p>
          </div>
        )}
      </div>
    </div>
  );
}
