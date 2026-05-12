const { exec } = require('child_process');
const util = require('util');
const execPromise = util.promisify(exec);
const fs = require('fs');

const PB_BINARY = process.env.PB_BINARY || '/opt/pocketbase/pocketbase';

async function startPocketBase(slug, port, dataPath) {
  try {
    if (!fs.existsSync(PB_BINARY)) {
      throw new Error(`PocketBase binary not found at ${PB_BINARY}. Please download it from https://github.com/pocketbase/pocketbase/releases`);
    }

    const cmd = `pm2 start ${PB_BINARY} --name "pb-${slug}" -- serve --http="127.0.0.1:${port}" --dir="${dataPath}"`;
    await execPromise(cmd);
    await execPromise('pm2 save');
    return true;
  } catch (error) {
    console.error('PocketBase start error:', error);
    throw error;
  }
}

async function stopPocketBase(slug) {
  try {
    await execPromise(`pm2 stop "pb-${slug}"`);
    return true;
  } catch (error) {
    console.error('PocketBase stop error:', error);
    throw error;
  }
}

async function deletePocketBase(slug) {
  try {
    await execPromise(`pm2 delete "pb-${slug}"`);
    await execPromise('pm2 save');
    return true;
  } catch (error) {
    console.error('PocketBase delete error:', error);
    // Might not exist, ignore
    return false;
  }
}

async function getPocketBaseStatus(slug) {
  try {
    const { stdout } = await execPromise(`pm2 jlist`);
    const list = JSON.parse(stdout);
    const process = list.find(p => p.name === `pb-${slug}`);
    if (!process) return { status: 'stopped' };
    return {
      status: process.pm2_env.status,
      uptime: process.pm2_env.pm_uptime
    };
  } catch (error) {
    console.error('PocketBase status error:', error);
    throw error;
  }
}

module.exports = {
  startPocketBase,
  stopPocketBase,
  deletePocketBase,
  getPocketBaseStatus
};
