const express = require('express');
const router = express.Router();
const fs = require('fs');
const path = require('path');
const { db } = require('../lib/db');
const { startPocketBase, stopPocketBase, deletePocketBase, getPocketBaseStatus } = require('../lib/pocketbase');

const PB_DATA_ROOT = process.env.PB_DATA_ROOT || '/var/pb-data';
const PB_BASE_PORT = parseInt(process.env.PB_BASE_PORT || '8090', 10);

router.post('/:id', async (req, res) => {
  const projectId = req.params.id;

  try {
    const project = db.prepare('SELECT * FROM projects WHERE id = ?').get(projectId);
    if (!project) return res.status(404).json({ error: 'Project not found' });

    const existing = db.prepare('SELECT id FROM pocketbase_instances WHERE project_id = ?').get(projectId);
    if (existing) return res.status(400).json({ error: 'PocketBase already exists for this project' });

    const maxPortRecord = db.prepare('SELECT MAX(port) as maxPort FROM pocketbase_instances').get();
    const port = maxPortRecord.maxPort ? maxPortRecord.maxPort + 1 : PB_BASE_PORT;
    const dataPath = path.join(PB_DATA_ROOT, project.slug);

    db.prepare(`
      INSERT INTO pocketbase_instances (project_id, port, data_path)
      VALUES (?, ?, ?)
    `).run(projectId, port, dataPath);

    if (!fs.existsSync(dataPath)) {
      fs.mkdirSync(dataPath, { recursive: true });
    }

    res.status(201).json({ message: 'PocketBase instance added', port });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.delete('/:id', async (req, res) => {
  const projectId = req.params.id;

  try {
    const pb = db.prepare('SELECT * FROM pocketbase_instances WHERE project_id = ?').get(projectId);
    if (!pb) return res.status(404).json({ error: 'PocketBase instance not found' });
    const project = db.prepare('SELECT slug FROM projects WHERE id = ?').get(projectId);

    await deletePocketBase(project.slug);
    
    if (fs.existsSync(pb.data_path)) {
      fs.rmSync(pb.data_path, { recursive: true, force: true });
    }

    db.prepare('DELETE FROM pocketbase_instances WHERE id = ?').run(pb.id);
    res.json({ message: 'PocketBase instance deleted' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.post('/:id/start', async (req, res) => {
  try {
    const pb = db.prepare('SELECT * FROM pocketbase_instances WHERE project_id = ?').get(req.params.id);
    if (!pb) return res.status(404).json({ error: 'PocketBase not found' });
    const project = db.prepare('SELECT slug FROM projects WHERE id = ?').get(req.params.id);

    await startPocketBase(project.slug, pb.port, pb.data_path);
    db.prepare('UPDATE pocketbase_instances SET status = ? WHERE id = ?').run('running', pb.id);

    res.json({ message: 'PocketBase started' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.post('/:id/stop', async (req, res) => {
  try {
    const pb = db.prepare('SELECT * FROM pocketbase_instances WHERE project_id = ?').get(req.params.id);
    if (!pb) return res.status(404).json({ error: 'PocketBase not found' });
    const project = db.prepare('SELECT slug FROM projects WHERE id = ?').get(req.params.id);

    await stopPocketBase(project.slug);
    db.prepare('UPDATE pocketbase_instances SET status = ? WHERE id = ?').run('stopped', pb.id);

    res.json({ message: 'PocketBase stopped' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.get('/:id/status', async (req, res) => {
  try {
    const project = db.prepare('SELECT slug FROM projects WHERE id = ?').get(req.params.id);
    if (!project) return res.status(404).json({ error: 'Project not found' });

    const status = await getPocketBaseStatus(project.slug);
    res.json(status);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

module.exports = router;
