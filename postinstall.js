#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

const home = process.env.HOME;
const destDir = path.join(home, '.finwipe');
const destFile = path.join(destDir, 'nbfcs.yaml');

try {
  fs.mkdirSync(destDir, { recursive: true });
  const srcFile = path.join(__dirname, 'data', 'nbfcs.yaml');
  if (fs.existsSync(srcFile)) {
    fs.copyFileSync(srcFile, destFile);
    console.log('FinWipe: NBFC data initialized at ~/.finwipe/nbfcs.yaml');
  } else {
    console.log('FinWipe: NBFC data not found in package, skipping');
  }
} catch (err) {
  console.error('FinWipe postinstall error:', err.message);
}
