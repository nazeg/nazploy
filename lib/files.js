const fs = require('fs');
const path = require('path');

const WWW_ROOT = process.env.WWW_ROOT || '/var/www';

function getSafePath(targetPath) {
  let resolvedPath;
  if (targetPath.startsWith(WWW_ROOT)) {
    resolvedPath = path.resolve(targetPath);
  } else {
    resolvedPath = path.resolve(WWW_ROOT, targetPath.replace(/^\//, ''));
  }

  if (!resolvedPath.startsWith(path.resolve(WWW_ROOT))) {
    throw new Error('Access denied: Path traversal detected');
  }
  return resolvedPath;
}

function listDirectory(dirPath) {
  const safePath = getSafePath(dirPath);
  if (!fs.existsSync(safePath)) {
    throw new Error('Directory not found');
  }
  
  const items = fs.readdirSync(safePath, { withFileTypes: true });
  return items.map(item => ({
    name: item.name,
    isDirectory: item.isDirectory(),
    path: path.join(dirPath, item.name)
  })).sort((a, b) => {
    if (a.isDirectory === b.isDirectory) return a.name.localeCompare(b.name);
    return a.isDirectory ? -1 : 1;
  });
}

function readFile(filePath) {
  const safePath = getSafePath(filePath);
  if (!fs.existsSync(safePath)) {
    throw new Error('File not found');
  }
  return fs.readFileSync(safePath, 'utf-8');
}

function writeFile(filePath, content) {
  const safePath = getSafePath(filePath);
  fs.writeFileSync(safePath, content, 'utf-8');
}

function deleteItem(itemPath) {
  const safePath = getSafePath(itemPath);
  if (!fs.existsSync(safePath)) {
    throw new Error('Item not found');
  }
  
  if (fs.lstatSync(safePath).isDirectory()) {
    fs.rmSync(safePath, { recursive: true, force: true });
  } else {
    fs.unlinkSync(safePath);
  }
}

function makeDir(dirPath) {
  const safePath = getSafePath(dirPath);
  if (!fs.existsSync(safePath)) {
    fs.mkdirSync(safePath, { recursive: true });
  }
}

function renameItem(oldPath, newPath) {
  const safeOldPath = getSafePath(oldPath);
  const safeNewPath = getSafePath(newPath);
  
  if (!fs.existsSync(safeOldPath)) {
    throw new Error('Item not found');
  }
  
  fs.renameSync(safeOldPath, safeNewPath);
}

module.exports = {
  getSafePath,
  listDirectory,
  readFile,
  writeFile,
  deleteItem,
  makeDir,
  renameItem
};
