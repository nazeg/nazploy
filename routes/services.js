const express = require('express');
const router = express.Router();
const { db } = require('../lib/db');
const { startProcess, stopProcess, restartProcess, getStatus, getOverview } = require('../lib/pm2');

router.post('/:id/start', async (req, res) => {
  try {
    const project = db.prepare('SELECT * FROM projects WHERE id = ?').get(req.params.id);
    if (!project) return res.status(404).json({ error: 'Project not found' });
    if (project.type === 'static') return res.status(400).json({ error: 'Static projects do not use PM2' });

    await startProcess(project.slug, project.entry_file, project.root_path);
    db.prepare('UPDATE projects SET status = ? WHERE id = ?').run('running', project.id);
    
    res.json({ message: 'Service started' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.post('/:id/stop', async (req, res) => {
  try {
    const project = db.prepare('SELECT * FROM projects WHERE id = ?').get(req.params.id);
    if (!project) return res.status(404).json({ error: 'Project not found' });

    await stopProcess(project.slug);
    db.prepare('UPDATE projects SET status = ? WHERE id = ?').run('stopped', project.id);

    res.json({ message: 'Service stopped' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.post('/:id/restart', async (req, res) => {
  try {
    const project = db.prepare('SELECT * FROM projects WHERE id = ?').get(req.params.id);
    if (!project) return res.status(404).json({ error: 'Project not found' });

    await restartProcess(project.slug);
    db.prepare('UPDATE projects SET status = ? WHERE id = ?').run('running', project.id);

    res.json({ message: 'Service restarted' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.get('/:id/status', async (req, res) => {
  try {
    const project = db.prepare('SELECT slug FROM projects WHERE id = ?').get(req.params.id);
    if (!project) return res.status(404).json({ error: 'Project not found' });

    const status = await getStatus(project.slug);
    res.json(status);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.get('/overview', async (req, res) => {
  try {
    const overview = await getOverview();
    res.json(overview);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

module.exports = router;
