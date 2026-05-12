const { exec } = require('child_process');
const util = require('util');
const execPromise = util.promisify(exec);

async function startProcess(slug, entryFile, cwd) {
  try {
    // Sanitize inputs implicitly or explicitly (we use safe slug characters)
    const cmd = `pm2 start ${entryFile} --name ${slug} --cwd ${cwd}`;
    const { stdout, stderr } = await execPromise(cmd);
    await execPromise('pm2 save');
    return { stdout, stderr };
  } catch (error) {
    console.error('PM2 start error:', error);
    throw error;
  }
}

async function stopProcess(slug) {
  try {
    const { stdout, stderr } = await execPromise(`pm2 stop ${slug}`);
    return { stdout, stderr };
  } catch (error) {
    console.error('PM2 stop error:', error);
    throw error;
  }
}

async function restartProcess(slug) {
  try {
    const { stdout, stderr } = await execPromise(`pm2 restart ${slug}`);
    return { stdout, stderr };
  } catch (error) {
    console.error('PM2 restart error:', error);
    throw error;
  }
}

async function deleteProcess(slug) {
  try {
    const { stdout, stderr } = await execPromise(`pm2 delete ${slug}`);
    await execPromise('pm2 save');
    return { stdout, stderr };
  } catch (error) {
    console.error('PM2 delete error:', error);
    throw error;
  }
}

async function getStatus(slug) {
  try {
    const { stdout } = await execPromise(`pm2 jlist`);
    const list = JSON.parse(stdout);
    const process = list.find(p => p.name === slug);
    if (!process) return { status: 'stopped' };
    return {
      status: process.pm2_env.status,
      cpu: process.monit.cpu,
      memory: process.monit.memory,
      uptime: process.pm2_env.pm_uptime
    };
  } catch (error) {
    console.error('PM2 status error:', error);
    throw error;
  }
}

async function getOverview() {
  try {
    const { stdout } = await execPromise(`pm2 jlist`);
    const list = JSON.parse(stdout);
    return list.map(p => ({
      name: p.name,
      status: p.pm2_env.status,
      cpu: p.monit.cpu,
      memory: p.monit.memory,
      uptime: p.pm2_env.pm_uptime
    }));
  } catch (error) {
    console.error('PM2 overview error:', error);
    throw error;
  }
}

async function getLogs(slug) {
  try {
    const { stdout } = await execPromise(`pm2 logs ${slug} --lines 100 --nostream`);
    return stdout;
  } catch (error) {
    console.error('PM2 logs error:', error);
    throw error;
  }
}

module.exports = {
  startProcess,
  stopProcess,
  restartProcess,
  deleteProcess,
  getStatus,
  getOverview,
  getLogs
};
