// needlescope cockpit frontend engine
(function () {
  'use strict';

  let currentReport = null;
  let eventSource = null;
  let hoveredCell = null;

  // DOM Elements
  const canvas = document.getElementById('matrix-canvas');
  const ctx = canvas.getContext('2d');
  const tooltip = document.getElementById('matrix-tooltip');
  const ucurveSvg = document.getElementById('ucurve-svg');

  // Metrics
  const metricOverall = document.getElementById('metric-overall');
  const metricMiddle = document.getElementById('metric-middle');
  const metricEdges = document.getElementById('metric-edges');
  const metricDelta = document.getElementById('metric-delta');
  const statusBadge = document.getElementById('status-badge');
  const statusText = document.getElementById('status-text');

  // U-Curve stats
  const valAlpha = document.getElementById('val-alpha');
  const valR2 = document.getElementById('val-r2');
  const blindspotBadge = document.getElementById('blindspot-badge');
  const valBlindspot = document.getElementById('val-blindspot');

  // Inspector
  const inspectCoords = document.getElementById('inspect-coords');
  const inspectContent = document.getElementById('inspector-content');

  // Modal
  const modal = document.getElementById('benchmark-modal');
  const btnOpenModal = document.getElementById('btn-open-benchmark');
  const btnCloseModal = document.getElementById('btn-close-modal');
  const btnCancelModal = document.getElementById('btn-cancel-modal');
  const btnStartBenchmark = document.getElementById('btn-start-benchmark');
  const btnStopBenchmark = document.getElementById('btn-stop-benchmark');
  const progressBox = document.getElementById('benchmark-progress-box');
  const progressFill = document.getElementById('bm-progress-fill');
  const progressText = document.getElementById('bm-progress-text');
  const progressPercent = document.getElementById('bm-progress-percent');
  const bmProvider = document.getElementById('bm-provider');
  const openaiOptions = document.getElementById('openai-options');

  // Playground
  const playQuery = document.getElementById('play-query');
  const playContext = document.getElementById('play-context');
  const playStrategy = document.getElementById('play-strategy');
  const btnRunRearrange = document.getElementById('btn-run-rearrange');
  const rearrangeResults = document.getElementById('rearrange-results');
  const resChunkCount = document.getElementById('res-chunk-count');
  const resEvictedCount = document.getElementById('res-evicted-count');
  const resRecallGain = document.getElementById('res-recall-gain');
  const chunksTimeline = document.getElementById('chunks-timeline');

  // Exports
  const btnExportJson = document.getElementById('btn-export-json');
  const btnExportCsv = document.getElementById('btn-export-csv');

  // Load initial dataset
  async function loadInitialReport() {
    try {
      const resp = await fetch('/api/matrix');
      if (resp.ok) {
        currentReport = await resp.json();
        updateUI();
      }
    } catch (err) {
      console.error('Failed to load matrix report:', err);
    }
  }

  function updateUI() {
    if (!currentReport) return;

    // Header metrics
    metricOverall.textContent = (currentReport.overall_recall * 100).toFixed(1) + '%';
    metricMiddle.textContent = (currentReport.middle_recall * 100).toFixed(1) + '%';
    metricEdges.textContent = `${(currentReport.primacy_recall * 100).toFixed(0)}% / ${(currentReport.recency_recall * 100).toFixed(0)}%`;
    metricDelta.textContent = (currentReport.degradation_delta * 100).toFixed(1) + '%';

    // U-Curve metrics
    if (currentReport.global_u_curve) {
      valAlpha.textContent = currentReport.global_u_curve.alpha.toFixed(3);
      valR2.textContent = currentReport.global_u_curve.r_squared.toFixed(2);
    }

    if (currentReport.critical_blind_spot && currentReport.critical_blind_spot.detected) {
      blindspotBadge.classList.remove('hidden');
      valBlindspot.textContent = `[${(currentReport.critical_blind_spot.min_depth * 100).toFixed(0)}% .. ${(currentReport.critical_blind_spot.max_depth * 100).toFixed(0)}%]`;
    } else {
      blindspotBadge.classList.add('hidden');
    }

    renderCanvasMatrix();
    renderUCurve();
  }

  // --- Canvas 2D Heatmap Rendering ---
  function renderCanvasMatrix() {
    if (!currentReport || !currentReport.context_lengths || !currentReport.depths) return;

    const lengths = currentReport.context_lengths;
    const depths = currentReport.depths;

    const dpr = window.devicePixelRatio || 1;
    const paddingLeft = 65;
    const paddingBottom = 40;
    const paddingTop = 25;
    const paddingRight = 20;

    const width = 640;
    const height = 320;

    canvas.width = width * dpr;
    canvas.height = height * dpr;
    canvas.style.width = width + 'px';
    canvas.style.height = height + 'px';

    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, width, height);

    const plotW = width - paddingLeft - paddingRight;
    const plotH = height - paddingTop - paddingBottom;

    const numCols = depths.length;
    const numRows = lengths.length;

    const cellW = plotW / numCols;
    const cellH = plotH / numRows;

    // Draw Column Headers (Depths)
    ctx.fillStyle = '#64748b';
    ctx.font = '10px -apple-system, sans-serif';
    ctx.textAlign = 'center';

    depths.forEach((d, i) => {
      const x = paddingLeft + i * cellW + cellW / 2;
      ctx.fillText(`${(d * 100).toFixed(0)}%`, x, paddingTop - 8);
    });

    // Draw Row Headers (Context Lengths)
    ctx.textAlign = 'right';
    lengths.forEach((l, i) => {
      const y = paddingTop + i * cellH + cellH / 2 + 4;
      const label = formatLength(l);
      ctx.fillText(label, paddingLeft - 10, y);
    });

    // Draw Cells
    lengths.forEach((l, r) => {
      depths.forEach((d, c) => {
        const x = paddingLeft + c * cellW;
        const y = paddingTop + r * cellH;

        const key = `${l}_${d.toFixed(2)}`;
        const cell = currentReport.cells[key];

        let fillColor = '#1c2842'; // default empty
        let recall = 0;

        if (cell) {
          recall = cell.recall_rate;
          fillColor = getColorForRecall(recall);
        }

        ctx.fillStyle = fillColor;
        ctx.fillRect(x + 1, y + 1, cellW - 2, cellH - 2);

        // Highlight hovered cell
        if (hoveredCell && hoveredCell.length === l && Math.abs(hoveredCell.depth - d) < 0.001) {
          ctx.strokeStyle = '#ffffff';
          ctx.lineWidth = 2;
          ctx.strokeRect(x, y, cellW, cellH);
        }

        // Draw percentage text if cell is large enough
        if (cell && cellW > 35) {
          ctx.fillStyle = recall >= 0.7 ? '#041019' : '#ffffff';
          ctx.font = 'bold 10px monospace';
          ctx.textAlign = 'center';
          ctx.fillText(`${(recall * 100).toFixed(0)}%`, x + cellW / 2, y + cellH / 2 + 3);
        }
      });
    });

    // Draw axis lines
    ctx.strokeStyle = '#1c2842';
    ctx.lineWidth = 1;
    ctx.strokeRect(paddingLeft, paddingTop, plotW, plotH);
  }

  function getColorForRecall(r) {
    if (r >= 0.95) return '#10b981'; // Green
    if (r >= 0.70) return '#f59e0b'; // Amber
    if (r >= 0.50) return '#f97316'; // Orange
    return '#ef4444'; // Red
  }

  function formatLength(n) {
    if (n >= 1024 * 1024) return `${n / (1024 * 1024)}M`;
    if (n >= 1024) return `${n / 1024}k`;
    return `${n}t`;
  }

  // Canvas Mouse Interaction
  canvas.addEventListener('mousemove', function (e) {
    if (!currentReport || !currentReport.context_lengths || !currentReport.depths) return;

    const rect = canvas.getBoundingClientRect();
    const mouseX = e.clientX - rect.left;
    const mouseY = e.clientY - rect.top;

    const paddingLeft = 65;
    const paddingBottom = 40;
    const paddingTop = 25;
    const paddingRight = 20;

    const width = 640;
    const height = 320;
    const plotW = width - paddingLeft - paddingRight;
    const plotH = height - paddingTop - paddingBottom;

    if (mouseX < paddingLeft || mouseX > paddingLeft + plotW || mouseY < paddingTop || mouseY > paddingTop + plotH) {
      tooltip.classList.add('hidden');
      hoveredCell = null;
      renderCanvasMatrix();
      return;
    }

    const cellW = plotW / currentReport.depths.length;
    const cellH = plotH / currentReport.context_lengths.length;

    const colIdx = Math.floor((mouseX - paddingLeft) / cellW);
    const rowIdx = Math.floor((mouseY - paddingTop) / cellH);

    if (colIdx >= 0 && colIdx < currentReport.depths.length && rowIdx >= 0 && rowIdx < currentReport.context_lengths.length) {
      const length = currentReport.context_lengths[rowIdx];
      const depth = currentReport.depths[colIdx];
      const key = `${length}_${depth.toFixed(2)}`;
      const cell = currentReport.cells[key];

      hoveredCell = { length, depth };
      renderCanvasMatrix();

      // Show Tooltip
      tooltip.classList.remove('hidden');
      tooltip.style.left = `${e.clientX - rect.left + 15}px`;
      tooltip.style.top = `${e.clientY - rect.top + 10}px`;

      if (cell) {
        tooltip.innerHTML = `
          <div><strong>Context:</strong> ${formatLength(cell.context_length)} tokens</div>
          <div><strong>Depth:</strong> ${(cell.depth * 100).toFixed(0)}%</div>
          <div><strong>Recall Rate:</strong> ${(cell.recall_rate * 100).toFixed(1)}% (${cell.successful_trials}/${cell.total_trials})</div>
          <div><strong>Latency:</strong> ${cell.mean_latency_ms.toFixed(0)}ms</div>
        `;
        renderInspector(cell);
      } else {
        tooltip.innerHTML = `<div>No trials executed for this coordinate</div>`;
      }
    }
  });

  canvas.addEventListener('mouseleave', function () {
    tooltip.classList.add('hidden');
    hoveredCell = null;
    renderCanvasMatrix();
  });

  function renderInspector(cell) {
    inspectCoords.textContent = `${formatLength(cell.context_length)} @ ${(cell.depth * 100).toFixed(0)}% depth`;

    const sample = cell.samples && cell.samples.length > 0 ? cell.samples[0] : null;

    inspectContent.innerHTML = `
      <div class="inspector-grid">
        <div class="inspect-metric">
          <div class="inspect-metric-label">Recall Success</div>
          <div class="inspect-metric-val ${cell.recall_rate >= 0.7 ? 'text-green' : 'text-red'}">
            ${(cell.recall_rate * 100).toFixed(1)}%
          </div>
        </div>
        <div class="inspect-metric">
          <div class="inspect-metric-label">Mean Latency</div>
          <div class="inspect-metric-val text-cyan">${cell.mean_latency_ms.toFixed(1)} ms</div>
        </div>
        <div class="inspect-metric">
          <div class="inspect-metric-label">P95 Latency</div>
          <div class="inspect-metric-val text-amber">${cell.p95_latency_ms.toFixed(1)} ms</div>
        </div>
        <div class="inspect-metric">
          <div class="inspect-metric-label">Average Score</div>
          <div class="inspect-metric-val">${(cell.average_score * 100).toFixed(1)}%</div>
        </div>
      </div>
      ${sample ? `
        <div class="inspect-sample-box">
          <div><strong style="color:var(--text-primary);">Expected:</strong> ${escapeHtml(sample.expected)}</div>
          <div style="margin-top:4px;"><strong style="color:var(--text-primary);">Response:</strong> ${escapeHtml(sample.actual)}</div>
        </div>
      ` : ''}
    `;
  }

  // --- U-Curve SVG Chart Rendering ---
  function renderUCurve() {
    if (!currentReport || !currentReport.depths) return;

    const depths = currentReport.depths;
    const svgW = 760;
    const svgH = 160;
    const padL = 45;
    const padR = 25;
    const padT = 15;
    const padB = 25;

    const plotW = svgW - padL - padR;
    const plotH = svgH - padT - padB;

    let svgHtml = '';

    // Draw Grid Lines (Y: 0%, 50%, 100%)
    [0, 0.5, 1.0].forEach(val => {
      const y = padT + (1.0 - val) * plotH;
      svgHtml += `<line x1="${padL}" y1="${y}" x2="${svgW - padR}" y2="${y}" stroke="#1c2842" stroke-dasharray="3 3"/>`;
      svgHtml += `<text x="${padL - 8}" y="${y + 4}" fill="#64748b" font-size="10" text-anchor="end">${(val * 100).toFixed(0)}%</text>`;
    });

    // Shaded Blind Spot (if detected)
    if (currentReport.critical_blind_spot && currentReport.critical_blind_spot.detected) {
      const bs = currentReport.critical_blind_spot;
      const x1 = padL + bs.min_depth * plotW;
      const x2 = padL + bs.max_depth * plotW;
      svgHtml += `<rect x="${x1}" y="${padT}" width="${x2 - x1}" height="${plotH}" fill="rgba(239, 68, 68, 0.12)" />`;
      svgHtml += `<text x="${(x1 + x2) / 2}" y="${padT + 14}" fill="#ef4444" font-size="9" text-anchor="middle" font-weight="bold">BLIND SPOT</text>`;
    }

    // Collect empirical average recall per depth
    const points = [];
    depths.forEach(d => {
      let sum = 0;
      let count = 0;
      currentReport.context_lengths.forEach(l => {
        const key = `${l}_${d.toFixed(2)}`;
        const c = currentReport.cells[key];
        if (c) {
          sum += c.recall_rate;
          count++;
        }
      });
      const avg = count > 0 ? sum / count : 1.0;
      const px = padL + d * plotW;
      const py = padT + (1.0 - avg) * plotH;
      points.push({ x: px, y: py, recall: avg, depth: d });
    });

    // Draw empirical line path
    if (points.length > 1) {
      let pathD = `M ${points[0].x} ${points[0].y}`;
      for (let i = 1; i < points.length; i++) {
        pathD += ` L ${points[i].x} ${points[i].y}`;
      }
      svgHtml += `<path d="${pathD}" fill="none" stroke="#06b6d4" stroke-width="2.5" />`;

      // Draw points
      points.forEach(pt => {
        const color = getColorForRecall(pt.recall);
        svgHtml += `<circle cx="${pt.x}" cy="${pt.y}" r="4" fill="${color}" stroke="#090d16" stroke-width="1.5" />`;
      });
    }

    // Draw X-axis Depth Labels
    depths.forEach(d => {
      const x = padL + d * plotW;
      svgHtml += `<text x="${x}" y="${svgH - 6}" fill="#64748b" font-size="10" text-anchor="middle">${(d * 100).toFixed(0)}%</text>`;
    });

    ucurveSvg.innerHTML = svgHtml;
  }

  // --- SSE Real-time Feed ---
  function connectSSE() {
    if (eventSource) {
      eventSource.close();
    }

    eventSource = new EventSource('/api/benchmark/stream');

    eventSource.onmessage = function (e) {
      try {
        const ev = JSON.parse(e.data);
        handleStreamEvent(ev);
      } catch (err) {
        console.error('SSE JSON parse error:', err);
      }
    };

    eventSource.onerror = function () {
      statusBadge.classList.remove('streaming');
      statusText.textContent = 'Disconnected';
    };
  }

  function handleStreamEvent(ev) {
    if (ev.type === 'start') {
      statusBadge.classList.add('streaming');
      statusText.textContent = 'Running Sweep...';
      progressBox.classList.remove('hidden');
      progressFill.style.width = '0%';
      btnStartBenchmark.classList.add('hidden');
      btnStopBenchmark.classList.remove('hidden');
    } else if (ev.type === 'trial_complete') {
      const pct = Math.floor((ev.completed / ev.total_trials) * 100);
      progressFill.style.width = `${pct}%`;
      progressPercent.textContent = `${pct}%`;
      progressText.textContent = `Completed ${ev.completed} / ${ev.total_trials} trials (${formatLength(ev.current_length)} @ ${(ev.current_depth * 100).toFixed(0)}%)`;

      // Live cell update
      if (ev.latest_trial && currentReport) {
        const t = ev.latest_trial;
        const key = `${t.context_length}_${t.depth.toFixed(2)}`;
        let cell = currentReport.cells[key];
        if (!cell) {
          cell = {
            context_length: t.context_length,
            depth: t.depth,
            total_trials: 0,
            successful_trials: 0,
            recall_rate: 0,
            average_score: 0,
            mean_latency_ms: 0,
            p95_latency_ms: 0,
            samples: []
          };
          currentReport.cells[key] = cell;
        }
        cell.total_trials++;
        if (t.success) cell.successful_trials++;
        cell.recall_rate = cell.successful_trials / cell.total_trials;
        cell.mean_latency_ms = t.latency_ms;
        cell.samples = [{
          expected: t.expected,
          actual: t.actual,
          success: t.success,
          latency_ms: t.latency_ms
        }];
        renderCanvasMatrix();
      }
    } else if (ev.type === 'finished') {
      statusBadge.classList.remove('streaming');
      statusText.textContent = 'Sweep Complete';
      progressFill.style.width = '100%';
      progressPercent.textContent = '100%';
      progressText.textContent = 'All trials completed.';
      btnStartBenchmark.classList.remove('hidden');
      btnStopBenchmark.classList.add('hidden');
      loadInitialReport();
    } else if (ev.type === 'error') {
      statusBadge.classList.remove('streaming');
      statusText.textContent = 'Error';
      btnStartBenchmark.classList.remove('hidden');
      btnStopBenchmark.classList.add('hidden');
    }
  }

  // --- Modal & Benchmark Controls ---
  btnOpenModal.addEventListener('click', () => {
    modal.classList.remove('hidden');
  });

  btnCloseModal.addEventListener('click', () => {
    modal.classList.add('hidden');
  });

  btnCancelModal.addEventListener('click', () => {
    modal.classList.add('hidden');
  });

  bmProvider.addEventListener('change', () => {
    if (bmProvider.value === 'openai') {
      openaiOptions.style.display = 'flex';
    } else {
      openaiOptions.style.display = 'none';
    }
  });

  btnStartBenchmark.addEventListener('click', async () => {
    const config = {
      provider: bmProvider.value,
      model: document.getElementById('bm-model').value || 'mock-evaluator-v1',
      endpoint: document.getElementById('bm-endpoint').value,
      api_key: document.getElementById('bm-apikey').value,
      type: document.getElementById('bm-type').value,
      domain: document.getElementById('bm-domain').value,
      trials_per_coordinate: parseInt(document.getElementById('bm-trials').value) || 2,
      concurrency: parseInt(document.getElementById('bm-concurrency').value) || 4,
      context_lengths: [8192, 16384, 32768, 65536, 131072, 262144],
      depths: [0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0]
    };

    try {
      const resp = await fetch('/api/benchmark/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      });
      if (resp.ok) {
        progressBox.classList.remove('hidden');
        btnStartBenchmark.classList.add('hidden');
        btnStopBenchmark.classList.remove('hidden');
      }
    } catch (err) {
      alert('Failed to initiate benchmark: ' + err.message);
    }
  });

  btnStopBenchmark.addEventListener('click', async () => {
    try {
      await fetch('/api/benchmark/stop', { method: 'POST' });
      btnStopBenchmark.classList.add('hidden');
      btnStartBenchmark.classList.remove('hidden');
    } catch (err) {
      console.error('Stop benchmark failed:', err);
    }
  });

  // --- Live Playground ---
  btnRunRearrange.addEventListener('click', async () => {
    const query = playQuery.value.trim();
    const text = playContext.value.trim();
    const strategy = playStrategy.value;

    if (!text) {
      alert('Please enter context text to reorder.');
      return;
    }

    try {
      btnRunRearrange.textContent = 'Processing...';
      const resp = await fetch('/api/rearrange', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          text: text,
          query: query,
          strategy: strategy,
          target_chunk_tokens: 150
        })
      });

      if (!resp.ok) throw new Error('Failed to process context');

      const res = await resp.json();
      rearrangeResults.classList.remove('hidden');
      resChunkCount.textContent = res.chunk_count;
      resEvictedCount.textContent = res.middle_valley_eviction_count;
      resRecallGain.textContent = `${(res.estimated_baseline_recall * 100).toFixed(0)}% → ${(res.estimated_reordered_recall * 100).toFixed(0)}%`;

      chunksTimeline.innerHTML = '';
      res.reordered_chunks.forEach((ch, idx) => {
        const card = document.createElement('div');
        card.className = 'chunk-card';

        const isSalient = ch.salience_score > 0.5;
        const inMiddle = ch.new_depth >= 0.35 && ch.new_depth <= 0.65;

        card.innerHTML = `
          <div style="flex:1; margin-right:12px;">
            <div class="chunk-pos">Slot #${idx + 1} (Depth: ${(ch.new_depth * 100).toFixed(0)}%) &bull; Orig Depth: ${(ch.original_depth * 100).toFixed(0)}%</div>
            <div style="color:var(--text-secondary); margin-top:2px;">${escapeHtml(ch.content.substring(0, 110))}...</div>
          </div>
          <div>
            <span class="chunk-score ${isSalient ? '' : 'low'}">BM25: ${ch.salience_score.toFixed(2)}</span>
          </div>
        `;
        chunksTimeline.appendChild(card);
      });
    } catch (err) {
      alert('Re-arrangement failed: ' + err.message);
    } finally {
      btnRunRearrange.textContent = 'Re-Order Context';
    }
  });

  // Example pre-fill for playground
  playQuery.value = "What is the Project Aegis confidential access key?";
  playContext.value = `Section 1. Consolidated operating margin projections for fiscal year 2027 reflect ongoing datacenter expansions.\n\nSection 2. Legal governance terms require mutual indemnification for data loss.\n\nCRITICAL RECORD: The confidential operational access key for Project Aegis is SEC-AEGIS-99214.\n\nSection 4. Technical specifications for distributed raft log replication.\n\nSection 5. Employee health and benefit plan enrollment guidelines.`;

  // Exports
  btnExportJson.addEventListener('click', () => {
    window.location.href = '/api/export/json';
  });

  btnExportCsv.addEventListener('click', () => {
    window.location.href = '/api/export/csv';
  });

  function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // Initialize
  loadInitialReport();
  connectSSE();
})();
