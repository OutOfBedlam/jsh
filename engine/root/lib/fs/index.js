'use strict';

/**
 * fs module - Node.js fs module compatible interface for jsh
 * 
 * This module provides filesystem operations similar to Node.js's fs module,
 * wrapping the native jsh filesystem API accessible via process.env.filesystem()
 */

const process = require('/lib/process');

// Get the native filesystem object
function getFS() {
    return process.env.filesystem();
}

// Get current working directory
function getCwd() {
    return process.env.get("PWD") || process.cwd();
}

// Resolve path to absolute path
function resolvePath(path) {
    if (path.startsWith("/")) {
        return path;
    }
    const cwd = getCwd();
    return cwd + (cwd.endsWith("/") ? "" : "/") + path;
}

// Convert byte array to string
function bytesToString(bytes) {
    return String.fromCharCode(...bytes);
}

// Convert string to byte array
function stringToBytes(str) {
    const bytes = [];
    for (let i = 0; i < str.length; i++) {
        bytes.push(str.charCodeAt(i));
    }
    return bytes;
}

/**
 * Read file contents synchronously
 * @param {string} path - File path
 * @param {object} options - Options (encoding: 'utf8' or null for buffer)
 * @returns {string|Array} File contents as string or byte array
 */
function readFileSync(path, options) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        const raw = fs.readFile(fullPath);
        
        // Default to utf8 encoding if not specified
        const encoding = options?.encoding || (typeof options === 'string' ? options : 'utf8');
        
        if (encoding === null || encoding === 'buffer') {
            return raw;
        }
        
        return bytesToString(raw);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, open '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Write file contents synchronously
 * @param {string} path - File path
 * @param {string|Array} data - Data to write (string or byte array)
 * @param {object} options - Options (encoding: 'utf8' or null for buffer)
 */
function writeFileSync(path, data, options) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        const encoding = options?.encoding || (typeof options === 'string' ? options : 'utf8');
        const bytes = (encoding === null || encoding === 'buffer' || Array.isArray(data)) 
            ? data 
            : stringToBytes(data);
        
        fs.writeFile(fullPath, bytes);
    } catch (e) {
        const error = new Error(`EACCES: permission denied, open '${path}'`);
        error.code = 'EACCES';
        error.errno = -13;
        error.path = path;
        throw error;
    }
}

/**
 * Append data to file synchronously
 * @param {string} path - File path
 * @param {string|Array} data - Data to append
 * @param {object} options - Options (encoding: 'utf8' or null for buffer)
 */
function appendFileSync(path, data, options) {
    try {
        const existing = readFileSync(path, { encoding: null });
        const encoding = options?.encoding || (typeof options === 'string' ? options : 'utf8');
        const newBytes = (encoding === null || encoding === 'buffer' || Array.isArray(data)) 
            ? data 
            : stringToBytes(data);
        
        writeFileSync(path, [...existing, ...newBytes], { encoding: null });
    } catch (e) {
        if (e.code === 'ENOENT') {
            // File doesn't exist, just write it
            writeFileSync(path, data, options);
        } else {
            throw e;
        }
    }
}

/**
 * Check if file or directory exists
 * @param {string} path - File or directory path
 * @returns {boolean} True if exists
 */
function existsSync(path) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        fs.stat(fullPath);
        return true;
    } catch (e) {
        return false;
    }
}

/**
 * Get file or directory stats
 * @param {string} path - File or directory path
 * @returns {object} Stats object with file information
 */
function statSync(path) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        const info = fs.stat(fullPath);
        const mode = info.mode();
        const modeStr = mode.string();
        
        return {
            isFile: () => !modeStr.startsWith('d') && !modeStr.startsWith('l'),
            isDirectory: () => modeStr.startsWith('d'),
            isSymbolicLink: () => modeStr.startsWith('l'),
            isBlockDevice: () => modeStr.startsWith('b'),
            isCharacterDevice: () => modeStr.startsWith('c'),
            isFIFO: () => modeStr.startsWith('p'),
            isSocket: () => modeStr.startsWith('s'),
            size: info.size(),
            mode: mode,
            mtime: info.modTime(),
            atime: info.modTime(), // jsh may not have separate atime
            ctime: info.modTime(), // jsh may not have separate ctime
            birthtime: info.modTime(),
            name: info.name()
        };
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, stat '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Get file or directory stats (follows symlinks)
 * @param {string} path - File or directory path
 * @returns {object} Stats object
 */
function lstatSync(path) {
    // For now, same as statSync since we don't have explicit lstat support
    return statSync(path);
}

/**
 * Read directory contents synchronously
 * @param {string} path - Directory path
 * @param {object} options - Options (withFileTypes: boolean)
 * @returns {Array} Array of filenames or Dirent objects
 */
function readdirSync(path, options) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        const entries = fs.readDir(fullPath);
        
        if (options?.withFileTypes) {
            // Return Dirent-like objects
            return entries.map((entry) => {
                const info = entry.info();
                const mode = info.mode();
                const modeStr = mode.string();
                
                return {
                    name: info.name(),
                    isFile: () => !modeStr.startsWith('d') && !modeStr.startsWith('l'),
                    isDirectory: () => modeStr.startsWith('d'),
                    isSymbolicLink: () => modeStr.startsWith('l'),
                    isBlockDevice: () => modeStr.startsWith('b'),
                    isCharacterDevice: () => modeStr.startsWith('c'),
                    isFIFO: () => modeStr.startsWith('p'),
                    isSocket: () => modeStr.startsWith('s')
                };
            });
        } else {
            // Return just filenames
            return entries.map((entry) => entry.info().name());
        }
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, scandir '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Create a directory synchronously
 * @param {string} path - Directory path
 * @param {object} options - Options (recursive: boolean, mode: number)
 */
function mkdirSync(path, options) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        if (options?.recursive) {
            // Create parent directories if needed
            const parts = fullPath.split('/').filter(p => p);
            let current = '/';
            
            for (const part of parts) {
                current += part + '/';
                try {
                    fs.mkdir(current.slice(0, -1));
                } catch (e) {
                    // Directory may already exist, continue
                }
            }
        } else {
            fs.mkdir(fullPath);
        }
    } catch (e) {
        if (!options?.recursive || !existsSync(path)) {
            const error = new Error(`EACCES: permission denied, mkdir '${path}'`);
            error.code = 'EACCES';
            error.errno = -13;
            error.path = path;
            throw error;
        }
    }
}

/**
 * Remove a directory synchronously
 * @param {string} path - Directory path
 * @param {object} options - Options (recursive: boolean)
 */
function rmdirSync(path, options) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    console.log('rmdirSync called for path:', fullPath);
    try {
        if (options?.recursive) {
            // Remove directory and all contents
            const entries = readdirSync(path, { withFileTypes: true });
            
            for (const entry of entries) {
                if (entry.name === '.' || entry.name === '..') {
                    continue;
                }
                const entryPath = path + '/' + entry.name;
                if (entry.isDirectory()) {
                    rmdirSync(entryPath, { recursive: true });
                } else {
                    unlinkSync(entryPath);
                }
            }
        }
        
        fs.rmdir(fullPath);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, rmdir '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Remove a file or directory synchronously (newer API)
 * @param {string} path - File or directory path
 * @param {object} options - Options (recursive: boolean, force: boolean)
 */
function rmSync(path, options) {
    try {
        const stats = statSync(path);
        if (stats.isDirectory()) {
            rmdirSync(path, options);
        } else {
            unlinkSync(path);
        }
    } catch (e) {
        if (!options?.force) {
            throw e;
        }
    }
}

/**
 * Delete a file synchronously
 * @param {string} path - File path
 */
function unlinkSync(path) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        fs.remove(fullPath);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, unlink '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Rename a file or directory synchronously
 * @param {string} oldPath - Old path
 * @param {string} newPath - New path
 */
function renameSync(oldPath, newPath) {
    const fs = getFS();
    const fullOldPath = resolvePath(oldPath);
    const fullNewPath = resolvePath(newPath);
    
    try {
        fs.rename(fullOldPath, fullNewPath);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, rename '${oldPath}' -> '${newPath}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = oldPath;
        throw error;
    }
}

/**
 * Copy a file synchronously
 * @param {string} src - Source path
 * @param {string} dest - Destination path
 * @param {number} flags - Copy flags (COPYFILE_EXCL, etc.)
 */
function copyFileSync(src, dest, flags) {
    const COPYFILE_EXCL = 1;
    
    // Check if destination exists when EXCL flag is set
    if (flags & COPYFILE_EXCL) {
        if (existsSync(dest)) {
            const error = new Error(`EEXIST: file already exists, copyfile '${src}' -> '${dest}'`);
            error.code = 'EEXIST';
            error.errno = -17;
            error.path = dest;
            throw error;
        }
    }
    
    const content = readFileSync(src, { encoding: null });
    writeFileSync(dest, content, { encoding: null });
}

/**
 * Change file permissions synchronously
 * @param {string} path - File path
 * @param {number|string} mode - Permission mode
 */
function chmodSync(path, mode) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        fs.chmod(fullPath, mode);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, chmod '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Change file owner synchronously
 * @param {string} path - File path
 * @param {number} uid - User ID
 * @param {number} gid - Group ID
 */
function chownSync(path, uid, gid) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        fs.chown(fullPath, uid, gid);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, chown '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Create a symbolic link synchronously
 * @param {string} target - Link target
 * @param {string} path - Link path
 */
function symlinkSync(target, path) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        fs.symlink(target, fullPath);
    } catch (e) {
        const error = new Error(`EACCES: permission denied, symlink '${target}' -> '${path}'`);
        error.code = 'EACCES';
        error.errno = -13;
        error.path = path;
        throw error;
    }
}

/**
 * Read a symbolic link synchronously
 * @param {string} path - Link path
 * @returns {string} Link target
 */
function readlinkSync(path) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        return fs.readlink(fullPath);
    } catch (e) {
        const error = new Error(`ENOENT: no such file or directory, readlink '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
}

/**
 * Get real path (resolving symlinks) synchronously
 * @param {string} path - Path to resolve
 * @returns {string} Real path
 */
function realpathSync(path) {
    const fs = getFS();
    const fullPath = resolvePath(path);
    
    try {
        // Try to resolve symlinks
        let current = fullPath;
        let visited = new Set();
        
        while (true) {
            if (visited.has(current)) {
                // Circular symlink
                const error = new Error(`ELOOP: too many symbolic links encountered, realpath '${path}'`);
                error.code = 'ELOOP';
                error.errno = -40;
                error.path = path;
                throw error;
            }
            
            visited.add(current);
            
            try {
                const stats = statSync(current);
                if (!stats.isSymbolicLink()) {
                    return current;
                }
                current = readlinkSync(current);
            } catch (e) {
                return current;
            }
        }
    } catch (e) {
        throw e;
    }
}

/**
 * Access check for file/directory
 * @param {string} path - Path to check
 * @param {number} mode - Access mode (F_OK, R_OK, W_OK, X_OK)
 */
function accessSync(path, mode) {
    const F_OK = 0; // File exists
    const R_OK = 4; // Read permission
    const W_OK = 2; // Write permission
    const X_OK = 1; // Execute permission
    
    if (!existsSync(path)) {
        const error = new Error(`ENOENT: no such file or directory, access '${path}'`);
        error.code = 'ENOENT';
        error.errno = -2;
        error.path = path;
        throw error;
    }
    
    // For simplicity, we assume if file exists, we have access
    // A real implementation would check actual permissions
}

/**
 * Truncate file to specified length
 * @param {string} path - File path
 * @param {number} len - Length to truncate to (default: 0)
 */
function truncateSync(path, len) {
    len = len || 0;
    
    if (len === 0) {
        writeFileSync(path, '', 'utf8');
    } else {
        const content = readFileSync(path, { encoding: null });
        if (content.length > len) {
            writeFileSync(path, content.slice(0, len), { encoding: null });
        }
    }
}

// Constants
const constants = {
    // File Access Constants
    F_OK: 0,
    R_OK: 4,
    W_OK: 2,
    X_OK: 1,
    
    // File Copy Constants
    COPYFILE_EXCL: 1,
    COPYFILE_FICLONE: 2,
    COPYFILE_FICLONE_FORCE: 4,
    
    // File Open Constants
    O_RDONLY: 0,
    O_WRONLY: 1,
    O_RDWR: 2,
    O_CREAT: 64,
    O_EXCL: 128,
    O_TRUNC: 512,
    O_APPEND: 1024,
};

// Export all functions
module.exports = {
    // File operations
    readFileSync,
    writeFileSync,
    appendFileSync,
    copyFileSync,
    unlinkSync,
    renameSync,
    truncateSync,
    
    // Directory operations
    readdirSync,
    mkdirSync,
    rmdirSync,
    rmSync,
    
    // File info
    statSync,
    lstatSync,
    existsSync,
    accessSync,
    
    // Symlink operations
    symlinkSync,
    readlinkSync,
    realpathSync,
    
    // Permissions
    chmodSync,
    chownSync,
    
    // Constants
    constants,
    
    // Aliases for Node.js compatibility
    readFile: readFileSync,
    writeFile: writeFileSync,
    appendFile: appendFileSync,
    copyFile: copyFileSync,
    unlink: unlinkSync,
    rename: renameSync,
    readdir: readdirSync,
    mkdir: mkdirSync,
    rmdir: rmdirSync,
    rm: rmSync,
    stat: statSync,
    lstat: lstatSync,
    exists: existsSync,
    access: accessSync,
    symlink: symlinkSync,
    readlink: readlinkSync,
    realpath: realpathSync,
    chmod: chmodSync,
    chown: chownSync,
    truncate: truncateSync
};
