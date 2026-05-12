const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const SITES_AVAILABLE = process.env.NGINX_SITES_AVAILABLE || '/etc/nginx/sites-available';
const SITES_ENABLED = process.env.NGINX_SITES_ENABLED || '/etc/nginx/sites-enabled';

function generateConfig(project) {
  const { type, domain, slug, port } = project;
  const rootPath = path.join(process.env.WWW_ROOT || '/var/www', slug);

  let config = '';

  if (type === 'static') {
    config = `
server {
    listen 80;
    server_name ${domain};
    root ${rootPath};
    index index.html;
    location / {
        try_files $uri $uri/ /index.html;
    }
}
`;
  } else if (type === 'node') {
    config = `
server {
    listen 80;
    server_name ${domain};
    location / {
        proxy_pass http://127.0.0.1:${port};
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
`;
  }

  return config.trim();
}

function saveAndEnableConfig(project) {
  try {
    const config = generateConfig(project);
    const configPath = path.join(SITES_AVAILABLE, project.slug);
    const symlinkPath = path.join(SITES_ENABLED, project.slug);

    fs.writeFileSync(configPath, config, 'utf-8');

    if (!fs.existsSync(symlinkPath)) {
      fs.symlinkSync(configPath, symlinkPath);
    }

    return configPath;
  } catch (error) {
    console.error('Nginx config save error:', error);
    throw error;
  }
}

function removeConfig(slug) {
  try {
    const configPath = path.join(SITES_AVAILABLE, slug);
    const symlinkPath = path.join(SITES_ENABLED, slug);

    if (fs.existsSync(symlinkPath)) fs.unlinkSync(symlinkPath);
    if (fs.existsSync(configPath)) fs.unlinkSync(configPath);
  } catch (error) {
    console.error('Nginx config remove error:', error);
    throw error;
  }
}

function reloadNginx() {
  try {
    execSync('sudo nginx -t && sudo systemctl reload nginx');
  } catch (error) {
    console.error('Nginx reload error:', error);
    throw error;
  }
}

module.exports = {
  saveAndEnableConfig,
  removeConfig,
  reloadNginx
};
