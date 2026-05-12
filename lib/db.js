const Database = require('better-sqlite3');
const path = require('path');

// Ensure db file is in the project root
const dbPath = path.join(__dirname, '..', 'dashboard.db');
const db = new Database(dbPath);

function initDb() {
  db.exec(`
    CREATE TABLE IF NOT EXISTS projects (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      name TEXT NOT NULL,
      slug TEXT UNIQUE NOT NULL,       
      domain TEXT,                     
      type TEXT NOT NULL,              
      root_path TEXT NOT NULL,         
      port INTEGER,                    
      entry_file TEXT,                 
      env_vars TEXT,                   
      status TEXT DEFAULT 'stopped',   
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS pocketbase_instances (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
      port INTEGER UNIQUE NOT NULL,    
      data_path TEXT NOT NULL,
      status TEXT DEFAULT 'stopped',
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS nginx_configs (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
      config_path TEXT NOT NULL,       
      is_active INTEGER DEFAULT 1,
      updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
  `);
}

module.exports = {
  db,
  initDb
};
