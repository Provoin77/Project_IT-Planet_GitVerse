const { Pool } = require('pg');

// Настройка подключения к БД
const pool = new Pool({
  user: 'gitverse_user',
  host: 'postgres', 
  database: 'gitverse_db',
  password: 'gitverse_password',
  port: 5432, 
});

// Функция для записи логов подключения в базу данных
const logToDatabase = async (message) => {
  try {
    await pool.query(`
      INSERT INTO task_log (task_id, log_type, message) 
      VALUES (NULL, 'Info', $1);
    `, [message]);
    console.log(`Database log recorded: ${message}`);
  } catch (error) {
    console.error('Failed to write log to database:', error);
  }
};

// Функция для генерации случайных логов
const generateRandomLog = () => {
  const logTypes = ['Info', 'Warning', 'Error'];
  const messages = [
    'Task started successfully.',
    'Minor warning occurred during execution.',
    'Progress saved.',
    'Task paused due to system maintenance.',
  ];

  // Вероятность ошибки 0.05
  const type = Math.random() < 0.95 ? logTypes[0] : logTypes[Math.floor(Math.random() * logTypes.length)];
  const message = messages[Math.floor(Math.random() * messages.length)];
  return { type, message };
};

// Основная функция для обновления задач
const updateTaskProgress = async () => {
  try {
    console.log('Attempting to update task progress...');
    await logToDatabase('Starting task progress update.');

    // 1. Поиск задачи со статусом Pending для перевода её в Running
    const { rows: pendingTasks } = await pool.query(`
      SELECT * FROM task WHERE status = 'Pending' LIMIT 1;
    `);

    if (pendingTasks.length > 0) {
      const task = pendingTasks[0];
      await pool.query(`
        UPDATE task SET status = 'Running', start_time = NOW() WHERE task_id = $1;
      `, [task.task_id]);

      console.log(`Task ${task.task_id} is now Running.`);
      await logToDatabase(`Task ${task.task_id} moved to Running.`);
    } else {
      console.log('No tasks with status Pending found.');
      await logToDatabase('No tasks with status Pending found.');
    }

    // 2. Поиск задач со статусом Running для обновления прогресса
    const { rows: runningTasks } = await pool.query(`
      SELECT * FROM task WHERE status = 'Running';
    `);

    if (runningTasks.length > 0) {
      for (const task of runningTasks) {
        const progress = Math.min((task.progress_percentage || 0) + 10, 100); // Увеличиваем прогресс на 10%
        const log = generateRandomLog(); // Генерируем случайный лог

        // Обновляем процент выполнения задачи
        await pool.query(`
          UPDATE task SET progress_percentage = $1 WHERE task_id = $2;
        `, [progress, task.task_id]);

        // Вставляем лог в таблицу логов задач
        await pool.query(`
          INSERT INTO task_log (task_id, log_type, message) VALUES ($1, $2, $3);
        `, [task.task_id, log.type, log.message]);

        console.log(`Task ${task.task_id}: Progress ${progress}%, Log: ${log.message}`);
        await logToDatabase(`Task ${task.task_id}: Progress ${progress}%, Log: ${log.message}`);

        // 3. Логика обработки ошибок и предупреждений
        
        if (log.type === 'Error') {
          // Перевод задачи в Failed
          await pool.query(`
            UPDATE task SET status = 'Failed' WHERE task_id = $1;
          `, [task.task_id]);
          console.log(`Task ${task.task_id} has Failed due to an error.`);
          await logToDatabase(`Task ${task.task_id} has Failed due to an error.`);
        } else if (log.type === 'Warning' && Math.random() < 0.2) { // Вероятность 20%
          // Перевод задачи в Pending
          await pool.query(`
            UPDATE task SET status = 'Pending' WHERE task_id = $1;
          `, [task.task_id]);
          console.log(`Task ${task.task_id} is now Pending due to a warning.`);
          await logToDatabase(`Task ${task.task_id} is now Pending due to a warning.`);
        }
       
      }
    } else {
      console.log('No tasks with status Running found.');
      await logToDatabase('No tasks with status Running found.');
    }
  } catch (error) {
    console.error('Error updating task progress:', error);
    await logToDatabase(`Error updating task progress: ${error.message}`);
  }
};

// Интервал выполнения mock-генератора
setInterval(updateTaskProgress, 10000);

// Тестирование подключения к базе данных при старте
(async () => {
  try {
    await pool.query('SELECT 1');
    console.log('Database connection successful.');
    await logToDatabase('Database connection successful.');
  } catch (error) {
    console.error('Database connection failed:', error);
    await logToDatabase(`Database connection failed: ${error.message}`);
  }
})();
