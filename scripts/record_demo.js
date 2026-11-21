const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const puppeteer = require('puppeteer');

(async () => {
  const framesDir = path.join(__dirname, '..', 'docs', 'images', 'frames');
  if (!fs.existsSync(framesDir)) {
    fs.mkdirSync(framesDir, { recursive: true });
  }

  console.log('Launching browser for authentic demo recording...');
  const browser = await puppeteer.launch({
    headless: 'new',
    defaultViewport: { width: 1280, height: 800 },
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });

  const page = await browser.newPage();
  await page.goto('http://localhost:8585', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('#matrix-canvas');

  // Helper to capture a frame
  let frameCount = 0;
  async function snap(name) {
    frameCount++;
    const pad = String(frameCount).padStart(3, '0');
    const fp = path.join(framesDir, `frame_${pad}.png`);
    await page.screenshot({ path: fp });
    console.log(`[Frame ${pad}] ${name}`);
  }

  // 1. Initial Overview
  await new Promise(r => setTimeout(r, 600));
  await snap('Initial Cockpit state');
  await snap('Initial Cockpit state hold');

  // 2. Hover over canvas coordinates
  const canvasElement = await page.$('#matrix-canvas');
  const box = await canvasElement.boundingBox();

  // Coordinates calculation:
  // paddingLeft: 70, paddingTop: 25, plotW: 550, plotH: 255
  // Row 0: 8k, Row 1: 16k, Row 2: 32k, Row 3: 64k, Row 4: 128k, Row 5: 262k
  // Depths 0.0 to 1.0 (11 columns)
  // Col 0: 0%, Col 5: 50%, Col 10: 100%

  // Point A: Primacy Zone (Row 4: 128k, Col 1: 10% depth) -> 100% recall
  const x1 = box.x + 70 + (550 / 11) * 1.5;
  const y1 = box.y + 25 + (255 / 6) * 4.5;
  await page.mouse.move(x1, y1);
  await new Promise(r => setTimeout(r, 400));
  await snap('Hover 128k @ 10% depth (Primacy 100% recall)');

  // Point B: Middle Valley Degradation (Row 4: 128k, Col 5: 50% depth) -> Lost in the Middle
  const x2 = box.x + 70 + (550 / 11) * 5.5;
  const y2 = box.y + 25 + (255 / 6) * 4.5;
  await page.mouse.move(x2, y2);
  await new Promise(r => setTimeout(r, 400));
  await snap('Hover 128k @ 50% depth (Middle valley degradation 45%)');
  await snap('Hover 128k @ 50% depth hold');

  // Point C: Deep context critical degradation (Row 5: 262k, Col 5: 50% depth) -> 38% recall
  const x3 = box.x + 70 + (550 / 11) * 5.5;
  const y3 = box.y + 25 + (255 / 6) * 5.5;
  await page.mouse.move(x3, y3);
  await new Promise(r => setTimeout(r, 400));
  await snap('Hover 262k @ 50% depth (Critical blind spot 38%)');

  // Point D: Recency Zone (Row 5: 262k, Col 10: 100% depth) -> 95% recall
  const x4 = box.x + 70 + (550 / 11) * 10.2;
  const y4 = box.y + 25 + (255 / 6) * 5.5;
  await page.mouse.move(x4, y4);
  await new Promise(r => setTimeout(r, 400));
  await snap('Hover 262k @ 100% depth (Recency 95%)');

  // Move away from canvas to clear tooltip
  await page.mouse.move(box.x + 10, box.y - 40);
  await new Promise(r => setTimeout(r, 300));

  // 3. Open Benchmark Sweep Modal
  await page.click('#btn-open-benchmark');
  await new Promise(r => setTimeout(r, 350));
  await snap('Opened Benchmark Sweep Modal');

  // Select preset: Claude 3.5 Sonnet
  await page.select('#bm-preset', 'claude-3-5-sonnet');
  await new Promise(r => setTimeout(r, 350));
  await snap('Selected Frontier Preset Claude 3.5 Sonnet');

  // Close modal
  await page.click('#btn-close-modal');
  await new Promise(r => setTimeout(r, 350));
  await snap('Closed Benchmark Modal');

  // 4. Salience Re-Arranger Playground Demo
  await page.evaluate(() => {
    const playQuery = document.getElementById('play-query');
    playQuery.value = 'What is the Project Aegis confidential access key?';
  });
  await new Promise(r => setTimeout(r, 200));
  await snap('Playground Query pre-filled');

  // Click Re-Order Context button
  await page.click('#btn-run-rearrange');
  await new Promise(r => setTimeout(r, 600));
  await snap('Executed Semantic Salience Re-arrangement');
  await snap('Displaying U-Curve Mapped Chunks and 52% -> 96% Recall Gain');

  // Scroll down to show Attention Gradient Spectrum
  await page.evaluate(() => {
    window.scrollBy({ top: 120, behavior: 'instant' });
  });
  await new Promise(r => setTimeout(r, 300));
  await snap('Attention Gradient Spectrum Simulator Pins');
  await snap('Final Cockpit Overview Hold');

  await browser.close();
  console.log(`Captured ${frameCount} authentic execution frames.`);

  // Assemble GIF using FFmpeg
  const outputGif = path.join(__dirname, '..', 'docs', 'images', 'needlescope_demo.gif');
  console.log('Compiling animated GIF via FFmpeg with palettegen...');

  const ffmpegCmd = `ffmpeg -y -framerate 1.8 -i "${path.join(framesDir, 'frame_%03d.png')}" -vf "fps=10,scale=1000:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128:stats_mode=diff[p];[s1][p]paletteuse=dither=bayer:bayer_scale=3" "${outputGif}"`;
  execSync(ffmpegCmd, { stdio: 'inherit' });

  const stat = fs.statSync(outputGif);
  console.log(`✓ Animated demo GIF created at ${outputGif} (${(stat.size / 1024 / 1024).toFixed(2)} MB)`);

  // Clean up frames directory
  fs.rmSync(framesDir, { recursive: true, force: true });
  console.log('✓ Temporary frame images cleaned up.');
})().catch(err => {
  console.error('Error generating demo GIF:', err);
  process.exit(1);
});
