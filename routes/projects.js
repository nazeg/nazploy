const express = require('express');
const router = express.Router();
const fs = require('fs');
const path = require('path');
const { db } = require('../lib/db');
const { saveAndEnableConfig, removeConfig, reloadNginx } = require('../lib/nginx');
const { deleteProcess, getLogs } = require('../lib/pm2');

const WWW_ROOT = process.env.WWW_ROOT || '/var/www';

router.get('/', (req, res) => {
  try {
    const projects = db.prepare('SELECT * FROM projects ORDER BY created_at DESC').all();
    const pbInstances = db.prepare('SELECT project_id, port, status FROM pocketbase_instances').all();
    
    // Attach pb info if exists
    projects.forEach(p => {
      const pb = pbInstances.find(pb => pb.project_id === p.id);
      if (pb) p.pocketbase = pb;
    });

    res.json(projects);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.post('/', (req, res) => {
  const { name, slug, domain, type, port, entry_file, env_vars } = req.body;
  
  if (!name || !slug || !type) {
    return res.status(400).json({ error: 'Missing required fields' });
  }

  const rootPath = path.join(WWW_ROOT, slug);

  try {
    db.prepare(`
      INSERT INTO projects (name, slug, domain, type, root_path, port, entry_file, env_vars)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `).run(name, slug, domain || null, type, rootPath, port || null, entry_file || null, env_vars ? JSON.stringify(env_vars) : null);

    // Create directory
    if (!fs.existsSync(rootPath)) {
      fs.mkdirSync(rootPath, { recursive: true });
    }

    // Create nginx config ONLY if domain is provided
    if (domain) {
      saveAndEnableConfig({ slug, domain, type, port });
      try { reloadNginx(); } catch(e) { console.error('Initial nginx reload failed', e); }
    }

    res.status(201).json({ message: 'Project created successfully' });
  } catch (error) {
    if (error.code === 'SQLITE_CONSTRAINT_UNIQUE') {
      return res.status(400).json({ error: 'Slug already exists' });
    }
    res.status(500).json({ error: error.message });
  }
});

router.put('/:id', (req, res) => {
  const { name, domain, port, entry_file } = req.body;
  const projectId = req.params.id;

  try {
    const project = db.prepare('SELECT * FROM projects WHERE id = ?').get(projectId);
    if (!project) return res.status(404).json({ error: 'Project not found' });

    db.prepare(`
      UPDATE projects 
      SET name = ?, domain = ?, port = ?, entry_file = ?
      WHERE id = ?
    `).run(name || project.name, domain || project.domain, port || project.port, entry_file || project.entry_file, projectId);

    // If domain was added or changed, update Nginx
    const updatedProject = db.prepare('SELECT * FROM projects WHERE id = ?').get(projectId);
    if (updatedProject.domain) {
      saveAndEnableConfig(updatedProject);
      try { reloadNginx(); } catch(e) {}
    }

    res.json({ message: 'Project updated successfully' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.delete('/:id', async (req, res) => {
  const projectId = req.params.id;
  const confirm = req.query.confirm;

  if (confirm !== 'true') {
    return res.status(400).json({ error: 'Confirm flag required' });
  }

  try {
    const project = db.prepare('SELECT * FROM projects WHERE id = ?').get(projectId);
    if (!project) return res.status(404).json({ error: 'Project not found' });

    // Stop and delete PM2 process if exists
    await deleteProcess(project.slug).catch(() => {});

    // Remove nginx config
    removeConfig(project.slug);
    try { reloadNginx(); } catch(e) {}

    // Delete folder
    if (fs.existsSync(project.root_path)) {
      fs.rmSync(project.root_path, { recursive: true, force: true });
    }

    db.prepare('DELETE FROM projects WHERE id = ?').run(projectId);

    res.json({ message: 'Project deleted successfully' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

router.get('/:id/logs', async (req, res) => {
  try {
    const project = db.prepare('SELECT slug FROM projects WHERE id = ?').get(req.params.id);
    if (!project) return res.status(404).json({ error: 'Project not found' });

    const logs = await getLogs(project.slug);
    res.json({ logs });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

module.exports = router;
