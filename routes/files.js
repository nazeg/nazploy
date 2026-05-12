const express = require('express');
const router = express.Router();
const multer = require('multer');
const { getSafePath, listDirectory, readFile, writeFile, deleteItem, makeDir, renameItem } = require('../lib/files');

const storage = multer.diskStorage({
  destination: function (req, file, cb) {
    try {
      const targetPath = req.body.path || req.query.path;
      if (!targetPath) return cb(new Error('Path is required'));
      cb(null, getSafePath(targetPath));
    } catch (err) {
      cb(err);
    }
  },
  filename: function (req, file, cb) {
    cb(null, file.originalname);
  }
});

const upload = multer({ storage });

router.get('/', (req, res) => {
  try {
    const dirPath = req.query.path || '/';
    const items = listDirectory(dirPath);
    res.json(items);
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
});

router.get('/read', (req, res) => {
  try {
    const filePath = req.query.path;
    if (!filePath) return res.status(400).json({ error: 'Path required' });
    const content = readFile(filePath);
    res.send(content);
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
});

router.post('/write', (req, res) => {
  try {
    const { path, content } = req.body;
    if (!path || content === undefined) return res.status(400).json({ error: 'Path and content required' });
    writeFile(path, content);
    res.json({ message: 'File saved' });
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
});

router.post('/upload', upload.array('files'), (req, res) => {
  res.json({ message: 'Files uploaded successfully' });
});

router.delete('/', (req, res) => {
  try {
    const targetPath = req.query.path || req.body.path;
    if (!targetPath) return res.status(400).json({ error: 'Path required' });
    deleteItem(targetPath);
    res.json({ message: 'Item deleted' });
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
});

router.post('/mkdir', (req, res) => {
  try {
    const { path } = req.body;
    if (!path) return res.status(400).json({ error: 'Path required' });
    makeDir(path);
    res.json({ message: 'Directory created' });
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
});

router.post('/rename', (req, res) => {
  try {
    const { oldPath, newPath } = req.body;
    if (!oldPath || !newPath) return res.status(400).json({ error: 'oldPath and newPath required' });
    renameItem(oldPath, newPath);
    res.json({ message: 'Item renamed' });
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
});

module.exports = router;
