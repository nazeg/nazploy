require('dotenv').config();
const express = require('express');
const path = require('path');
const { initDb } = require('./lib/db');
const { router: authRouter, authenticateToken } = require('./routes/auth');
const projectsRouter = require('./routes/projects');
const servicesRouter = require('./routes/services');
const pocketbaseRouter = require('./routes/pocketbase');
const filesRouter = require('./routes/files');
const os = require('os');
const rateLimit = require('express-rate-limit');

const app = express();
const PORT = process.env.PORT || 3000;

// Initialize Database
initDb();

// Middleware
app.use(express.json());
app.use(express.static(path.join(__dirname, 'public')));

// Rate Limiter for login
const loginLimiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 10, // Limit each IP to 10 login requests per `window` (here, per 15 minutes)
  message: 'Too many login attempts, please try again later'
});

// Routes
app.use('/api/auth/login', loginLimiter);
app.use('/api/auth', authRouter);

// Protected routes
app.use('/api/projects', authenticateToken, projectsRouter);
app.use('/api/services', authenticateToken, servicesRouter);
app.use('/api/pocketbase', authenticateToken, pocketbaseRouter);
app.use('/api/files', authenticateToken, filesRouter);

// System stats route
app.get('/api/system/stats', authenticateToken, (req, res) => {
  const cpuLoad = os.loadavg();
  const totalMem = os.totalmem();
  const freeMem = os.freemem();
  const usedMem = totalMem - freeMem;
  res.json({
    cpu: cpuLoad,
    memory: {
      total: totalMem,
      free: freeMem,
      used: usedMem,
      usagePercent: ((usedMem / totalMem) * 100).toFixed(2)
    },
    uptime: os.uptime()
  });
});

// Serve frontend
app.get('/', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'index.html'));
});

app.get('/dashboard', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'dashboard.html'));
});

app.listen(PORT, () => {
  console.log(`Dashboard server running on http://localhost:${PORT}`);
});
