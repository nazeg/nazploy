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

      // Card
      let pbHtml = '';
      if (p.pocketbase) {
        pbHtml = `<div style="margin-top:0.5rem;font-size:0.8rem">PB Port: ${p.pocketbase.port} <button onclick="actionPB(${p.id}, '${p.pocketbase.status === 'running' ? 'stop' : 'start'}')" class="btn btn-outline btn-sm">${p.pocketbase.status === 'running' ? 'Stop PB' : 'Start PB'}</button> <button onclick="deletePB(${p.id})" class="btn btn-danger btn-sm">Del PB</button></div>`;
      } else {
        pbHtml = `<div style="margin-top:0.5rem"><button onclick="addPB(${p.id})" class="btn btn-outline btn-sm">Add PocketBase</button></div>`;
      }

      list.innerHTML += `
        <div class="card">
          <div class="card-header">
            <h3>${p.name}</h3>
            <span class="badge ${p.status}">${p.status}</span>
          </div>
          <div class="card-body">
            <div>Domain: ${p.domain || '<span class="text-secondary">Not set</span>'}</div>
            <div>Type: ${p.type}</div>
            ${p.type === 'node' ? `<div>Port: ${p.port}</div>` : ''}
            ${pbHtml}
          </div>
          <div class="card-actions">
            ${p.type === 'node' ? `
              <button onclick="actionProject(${p.id}, 'start')" class="btn btn-outline btn-sm">Start</button>
              <button onclick="actionProject(${p.id}, 'stop')" class="btn btn-outline btn-sm">Stop</button>
              <button onclick="actionProject(${p.id}, 'restart')" class="btn btn-outline btn-sm">Restart</button>
            ` : ''}
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
