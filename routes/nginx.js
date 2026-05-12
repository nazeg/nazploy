const express = require('express');
const router = express.Router();
const { reloadNginx } = require('../lib/nginx');

router.post('/reload', (req, res) => {
  try {
    reloadNginx();
    res.json({ message: 'Nginx reloaded successfully' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

module.exports = router;
