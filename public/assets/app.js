const token = localStorage.getItem('token');
if (!token) window.location.href = '/';

// Helpers
const fetchAPI = async (endpoint, options = {}) => {
  const res = await fetch(`/api${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
      ...options.headers
    }
  });
  if (res.status === 401 || res.status === 403) {
    localStorage.removeItem('token');
    window.location.href = '/';
  }
  return res;
};

// Navigation
document.querySelectorAll('.nav-item').forEach(item => {
  item.addEventListener('click', (e) => {
    e.preventDefault();
    document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
    document.querySelectorAll('.view-section').forEach(s => s.style.display = 'none');
    
    item.classList.add('active');
    document.getElementById(item.dataset.target).style.display = 'block';
  });
});

// Logout
document.getElementById('logout-btn').addEventListener('click', () => {
  localStorage.removeItem('token');
  window.location.href = '/';
});

// System Stats
const loadStats = async () => {
  try {
    const res = await fetchAPI('/system/stats');
    const data = await res.json();
    document.getElementById('stats-content').innerHTML = `
      CPU: ${data.cpu[0].toFixed(2)}<br>
      RAM: ${data.memory.usagePercent}%<br>
      Uptime: ${Math.floor(data.uptime / 3600)}h
    `;
  } catch (e) {
    console.error('Stats error', e);
  }
};
setInterval(loadStats, 10000);
loadStats();

// Projects
const loadProjects = async () => {
  try {
    const res = await fetchAPI('/projects');
    const projects = await res.json();
    
    const list = document.getElementById('projects-list');
    const projectSelect = document.getElementById('project-select');
    const logSelect = document.getElementById('log-project-select');
    
    list.innerHTML = '';
    projectSelect.innerHTML = '<option value="">Select Project</option>';
    logSelect.innerHTML = '<option value="">Select Project</option>';

    projects.forEach(p => {
      // Select options
      const opt = `<option value="${p.slug}">${p.name}</option>`;
      const optId = `<option value="${p.id}">${p.name}</option>`;
      projectSelect.innerHTML += opt;
      logSelect.innerHTML += optId;

      // Status & Link Logic
      const isStatic = p.type === 'static';
      const statusText = isStatic ? 'Active' : p.status;
      const statusClass = isStatic ? 'running' : p.status;
      
      // Link Construction
      const host = p.domain || window.location.hostname;
      const portSuffix = p.port && p.port !== 80 ? `:${p.port}` : '';
      const projectUrl = `http://${host}${portSuffix}`;

      // Card
      let pbHtml = '';
      if (p.pocketbase) {
        const pbIsRunning = p.pocketbase.status === 'running';
        const pbAdminUrl = `http://${host}:${p.pocketbase.port}/_/#/login`;
        
        pbHtml = `
          <div class="pb-instance-box ${pbIsRunning ? 'active' : ''}">
            <div class="pb-header">
              <span class="pb-title">PocketBase</span>
              <span class="pb-status-dot ${pbIsRunning ? 'online' : 'offline'}"></span>
              <span class="pb-port">Port: ${p.pocketbase.port}</span>
            </div>
            <div class="pb-actions">
              ${pbIsRunning ? `
                <a href="${pbAdminUrl}" target="_blank" class="btn btn-outline btn-sm pb-admin-btn">
                  Admin Panel ↗
                </a>
              ` : ''}
              <button onclick="actionPB(${p.id}, '${pbIsRunning ? 'stop' : 'start'}')" 
                      class="btn ${pbIsRunning ? 'btn-danger-soft' : 'btn-success-soft'} btn-sm">
                ${pbIsRunning ? 'Stop' : 'Start'}
              </button>
              <button onclick="deletePB(${p.id})" class="btn btn-icon-only btn-danger-soft btn-sm" title="Delete PB">
                🗑️
              </button>
            </div>
          </div>
        `;
      } else {
        pbHtml = `
          <div class="pb-empty-box">
            <button onclick="addPB(${p.id})" class="btn btn-outline btn-sm btn-full">
              + Add PocketBase Instance
            </button>
          </div>
        `;
      }

      list.innerHTML += `
        <div class="card">
          <div class="card-header">
            <h3>${p.name}</h3>
            <span class="badge ${statusClass}">${statusText}</span>
          </div>
          <div class="card-body">
            <div class="card-meta">
              <span class="meta-label">Domain:</span>
              <span class="meta-value">${p.domain || 'Not set'}</span>
            </div>
            <div class="card-meta">
              <span class="meta-label">Type:</span>
              <span class="meta-value">${p.type}</span>
            </div>
            <div style="margin-top: 15px;">
              <a href="${projectUrl}" target="_blank" class="btn btn-primary btn-sm btn-full">
                 Launch Site ↗
              </a>
            </div>
            ${pbHtml}
          </div>
          <div class="card-actions">
            ${p.type === 'node' ? `
              <button onclick="actionProject(${p.id}, 'start')" class="btn btn-outline btn-sm">Start</button>
              <button onclick="actionProject(${p.id}, 'stop')" class="btn btn-outline btn-sm">Stop</button>
              <button onclick="actionProject(${p.id}, 'restart')" class="btn btn-outline btn-sm">Restart</button>
            ` : ''}
            <div class="action-divider"></div>
            <button onclick="editProject(${p.id}, '${p.name}', '${p.domain || ''}', ${p.port || 'null'})" class="btn btn-outline btn-sm">Edit</button>
            <button onclick="deleteProject(${p.id})" class="btn btn-danger btn-sm">Delete</button>
          </div>
        </div>
      `;

    });
  } catch (e) {
    console.error('Projects load error', e);
  }
};

window.editProject = async (id, name, domain, port) => {
  const newDomain = prompt('Enter new domain:', domain);
  if (newDomain === null) return;
  const newName = prompt('Enter new name:', name);
  if (newName === null) return;

  const res = await fetchAPI(`/projects/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name: newName, domain: newDomain })
  });

  if (res.ok) {
    loadProjects();
  } else {
    alert('Update failed');
  }
};

window.actionProject = async (id, action) => {
  await fetchAPI(`/services/${id}/${action}`, { method: 'POST' });
  loadProjects();
};

window.deleteProject = async (id) => {
  if (confirm('Are you sure you want to delete this project?')) {
    await fetchAPI(`/projects/${id}?confirm=true`, { method: 'DELETE' });
    loadProjects();
  }
};

window.addPB = async (id) => {
  await fetchAPI(`/pocketbase/${id}`, { method: 'POST' });
  loadProjects();
};

window.actionPB = async (id, action) => {
  await fetchAPI(`/pocketbase/${id}/${action}`, { method: 'POST' });
  loadProjects();
};

window.deletePB = async (id) => {
  if (confirm('Delete PocketBase instance?')) {
    await fetchAPI(`/pocketbase/${id}`, { method: 'DELETE' });
    loadProjects();
  }
};

loadProjects();

// New Project Modal
const modal = document.getElementById('project-modal');
document.getElementById('new-project-btn').addEventListener('click', () => {
  modal.classList.add('open');
});
document.querySelector('.close-modal').addEventListener('click', () => {
  modal.classList.remove('open');
});

const typeSelect = document.getElementById('p-type');
typeSelect.addEventListener('change', () => {
  const isNode = typeSelect.value === 'node';
  document.getElementById('port-group').style.display = isNode ? 'block' : 'none';
  document.getElementById('entry-group').style.display = isNode ? 'block' : 'none';
});

document.getElementById('new-project-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const data = {
    name: document.getElementById('p-name').value,
    slug: document.getElementById('p-slug').value,
    domain: document.getElementById('p-domain').value,
    type: document.getElementById('p-type').value,
    port: document.getElementById('p-port').value,
    entry_file: document.getElementById('p-entry').value
  };

  const res = await fetchAPI('/projects', {
    method: 'POST',
    body: JSON.stringify(data)
  });

  if (res.ok) {
    modal.classList.remove('open');
    loadProjects();
  } else {
    const err = await res.json();
    alert(err.error || 'Failed to create project');
  }
});

// File Manager
let currentPath = '/var/www';

document.getElementById('project-select').addEventListener('change', (e) => {
  if (e.target.value) {
    currentPath = `/var/www/${e.target.value}`;
    loadFiles(currentPath);
  }
});

const loadFiles = async (pathStr) => {
  document.getElementById('current-path').textContent = pathStr;
  const res = await fetchAPI(`/files?path=${encodeURIComponent(pathStr)}`);
  if (!res.ok) return;
  const files = await res.json();
  
  const list = document.getElementById('file-list');
  list.innerHTML = '';
  
  if (pathStr !== '/var/www') {
    const parentPath = pathStr.split('/').slice(0, -1).join('/');
    list.innerHTML += `<div class="file-item" onclick="loadFiles('${parentPath}')"><span class="file-icon">📁</span> ..</div>`;
  }

  files.forEach(f => {
    const icon = f.isDirectory ? '📁' : '📄';
    const onclick = f.isDirectory ? `loadFiles('${f.path}')` : `openFile('${f.path}')`;
    list.innerHTML += `<div class="file-item" onclick="${onclick}"><span class="file-icon">${icon}</span> ${f.name}</div>`;
  });
};

window.openFile = async (pathStr) => {
  const res = await fetchAPI(`/files/read?path=${encodeURIComponent(pathStr)}`);
  if (res.ok) {
    const content = await res.text();
    document.getElementById('file-editor-container').style.display = 'flex';
    document.getElementById('editing-filename').textContent = pathStr.split('/').pop();
    document.getElementById('editing-filename').dataset.path = pathStr;
    document.getElementById('file-content').value = content;
  }
};

document.getElementById('close-editor-btn').addEventListener('click', () => {
  document.getElementById('file-editor-container').style.display = 'none';
});

document.getElementById('save-file-btn').addEventListener('click', async () => {
  const pathStr = document.getElementById('editing-filename').dataset.path;
  const content = document.getElementById('file-content').value;
  const res = await fetchAPI('/files/write', {
    method: 'POST',
    body: JSON.stringify({ path: pathStr, content })
  });
  if (res.ok) alert('File saved');
});

// Logs
let logInterval;
document.getElementById('log-project-select').addEventListener('change', (e) => {
  clearInterval(logInterval);
  if (e.target.value) {
    loadProjectLogs(e.target.value);
    logInterval = setInterval(() => loadProjectLogs(e.target.value), 5000);
  } else {
    document.getElementById('logs-content').textContent = 'Select a project to view logs...';
  }
});

const loadProjectLogs = async (projectId) => {
  const res = await fetchAPI(`/projects/${projectId}/logs`);
  if (res.ok) {
    const data = await res.json();
    document.getElementById('logs-content').textContent = data.logs || 'No logs available';
  }
};

// File Manager Uploads
const fileInput = document.getElementById('file-input');
const uploadBtn = document.getElementById('upload-btn');
const fileList = document.getElementById('file-list');

uploadBtn.addEventListener('click', () => {
  if (!document.getElementById('project-select').value) return alert('Please select a project first');
  fileInput.click();
});

fileInput.addEventListener('change', (e) => {
  handleUploads(e.target.files);
});

// Drag & Drop
fileList.addEventListener('dragover', (e) => {
  e.preventDefault();
  fileList.classList.add('drag-over');
});

fileList.addEventListener('dragleave', () => {
  fileList.classList.remove('drag-over');
});

fileList.addEventListener('drop', (e) => {
  e.preventDefault();
  fileList.classList.remove('drag-over');
  handleUploads(e.dataTransfer.files);
});

const handleUploads = async (files) => {
  if (files.length === 0) return;
  
  const formData = new FormData();
  formData.append('path', currentPath);
  for (let i = 0; i < files.length; i++) {
    formData.append('files', files[i]);
  }

  const res = await fetch('/api/files/upload', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`
    },
    body: formData
  });

  if (res.ok) {
    loadFiles(currentPath);
  } else {
    alert('Upload failed');
  }
};
