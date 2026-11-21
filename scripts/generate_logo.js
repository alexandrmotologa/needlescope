const fs = require('fs');
const path = require('path');
const { Resvg } = require('@resvg/resvg-js');

function buildLogoSvg() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">
  <defs>
    <!-- Squircle container clipping -->
    <clipPath id="squircle-clip">
      <rect x="24" y="24" width="976" height="976" rx="220" />
    </clipPath>

    <!-- Electric Cyan Optic & Telemetry Pulse Gradients -->
    <linearGradient id="cyan-glow" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#38bdf8"/>
      <stop offset="40%" stop-color="#00f5ff"/>
      <stop offset="100%" stop-color="#0284c7"/>
    </linearGradient>

    <!-- Precision Needle Amber Accent -->
    <linearGradient id="amber-reticle" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#fbbf24"/>
      <stop offset="50%" stop-color="#f59e0b"/>
      <stop offset="100%" stop-color="#ea580c"/>
    </linearGradient>

    <!-- High-Contrast Obsidian & Slate Lighting Gradients -->
    <linearGradient id="g-bright" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#94a3b8"/>
      <stop offset="100%" stop-color="#475569"/>
    </linearGradient>

    <linearGradient id="g-lit" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#64748b"/>
      <stop offset="100%" stop-color="#334155"/>
    </linearGradient>

    <linearGradient id="g-mid" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#334155"/>
      <stop offset="100%" stop-color="#1e293b"/>
    </linearGradient>

    <linearGradient id="g-shadow" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#1e293b"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>

    <linearGradient id="g-dark" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#0f172a"/>
      <stop offset="100%" stop-color="#070a11"/>
    </linearGradient>

    <filter id="subtle-shadow" x="-12%" y="-12%" width="124%" height="124%">
      <feDropShadow dx="0" dy="16" stdDeviation="22" flood-color="#070a12" flood-opacity="0.22" />
    </filter>
  </defs>

  <!-- Luxury White Squircle Container -->
  <rect x="24" y="24" width="976" height="976" rx="220" fill="#ffffff" stroke="#e2e8f0" stroke-width="6" />

  <g clip-path="url(#squircle-clip)">
    <g transform="translate(512, 512)" filter="url(#subtle-shadow)">

      <!-- Hexagonal Architectural Gateway Frame -->
      <polygon points="
        0,-400
        346,-200
        346,200
        0,400
        -346,200
        -346,-200
      " fill="none" stroke="#0f172a" stroke-width="36" stroke-linejoin="round" />

      <!-- Inner Cyan Telemetry Ring -->
      <polygon points="
        0,-365
        315,-182
        315,182
        0,365
        -315,182
        -315,-182
      " fill="none" stroke="#00f5ff" stroke-width="4" opacity="0.45" stroke-dasharray="16, 12" />

      <!-- ==================================================================== -->
      <!-- 1. BASE SOLID SILHOUETTE (100% Watertight - Zero Gap Protection)     -->
      <!-- ==================================================================== -->
      <polygon points="
        0,-245
        105,-235
        215,-325
        245,-180
        265,-55
        250,95
        175,175
        155,315
        0,340
        -155,315
        -175,175
        -250,95
        -265,-55
        -245,-180
        -215,-325
        -105,-235
      " fill="#080c14" />

      <!-- ==================================================================== -->
      <!-- 2. ORIGAMI LOW-POLY PLUMAGE (Precision Scope-Owl Noctua)              -->
      <!-- ==================================================================== -->

      <!-- Outer Ear Tufts -->
      <!-- Left Ear Tip -->
      <polygon points="-215,-325 -105,-235 -145,-165" fill="#1e293b" />
      <polygon points="-215,-325 -245,-180 -145,-165" fill="#0f172a" />
      <!-- Right Ear Tip -->
      <polygon points="215,-325 105,-235 145,-165" fill="#334155" />
      <polygon points="215,-325 245,-180 145,-165" fill="#1e293b" />

      <!-- Crown & Forehead Facets -->
      <polygon points="0,-245 -105,-235 -115,-155 0,-145" fill="#1e293b" />
      <polygon points="0,-245 105,-235 115,-155 0,-145" fill="#334155" />
      <polygon points="0,-145 -115,-155 0,-40" fill="#334155" />
      <polygon points="0,-145 115,-155 0,-40" fill="#475569" />

      <!-- Temples / Upper Cheeks -->
      <polygon points="-145,-165 -245,-180 -265,-55 -175,-5" fill="#0f172a" />
      <polygon points="145,-165 245,-180 265,-55 175,-5" fill="#1e293b" />
      <polygon points="-145,-165 -175,-5 -115,-155" fill="#1e293b" />
      <polygon points="145,-165 175,-5 115,-155" fill="#334155" />

      <!-- Zygomatic Cheeks & Lateral Feathers -->
      <polygon points="-265,-55 -250,95 -175,175 -175,-5" fill="#0f172a" />
      <polygon points="265,-55 250,95 175,175 175,-5" fill="#1e293b" />

      <!-- ==================================================================== -->
      <!-- 3. PARABOLIC FACIAL DISCS (Mathematical Attention Parabola Array)     -->
      <!-- ==================================================================== -->

      <!-- Left Parabolic Disc (Shadow / Depth) -->
      <polygon points="-115,-155 -175,-5 -115,45 -35,-20" fill="#1e293b" />
      <polygon points="-175,-5 -175,175 -85,235 -35,115" fill="#0f172a" />
      <polygon points="-175,-5 -35,115 -115,45" fill="#172033" />
      <polygon points="-35,-20 -115,45 -35,115" fill="#0b0f19" />

      <!-- Right Parabolic Disc (Volumetric Highlight) -->
      <polygon points="115,-155 175,-5 115,45 35,-20" fill="#334155" />
      <polygon points="175,-5 175,175 85,235 35,115" fill="#1e293b" />
      <polygon points="175,-5 35,115 115,45" fill="#2d3d57" />
      <polygon points="35,-20 115,45 35,115" fill="#141c2b" />

      <!-- Disc Parabolic Focus Ridges -->
      <line x1="-165" y1="5" x2="-85" y2="230" stroke="#00f5ff" stroke-width="2" opacity="0.4" />
      <line x1="165" y1="5" x2="85" y2="230" stroke="#00f5ff" stroke-width="2" opacity="0.4" />

      <!-- ==================================================================== -->
      <!-- 4. PREDATOR ATTENTION OPTICS (Electric Cyan Sockets & Slits)          -->
      <!-- ==================================================================== -->

      <!-- Left Eye Orbit Socket -->
      <polygon points="-135,-30 -60,-55 -38,-20 -105,5" fill="#080c14" stroke="#00f5ff" stroke-width="1.5" />
      <!-- Left Iris Glow -->
      <polygon points="-125,-28 -70,-48 -50,-20 -100,-2" fill="url(#cyan-glow)" />
      <!-- Left Vertical Slit Pupil -->
      <polygon points="-90,-42 -82,-40 -80,-8 -88,-10" fill="#04070d" />

      <!-- Right Eye Orbit Socket -->
      <polygon points="135,-30 60,-55 38,-20 105,5" fill="#080c14" stroke="#00f5ff" stroke-width="1.5" />
      <!-- Right Iris Glow -->
      <polygon points="125,-28 70,-48 50,-20 100,-2" fill="url(#cyan-glow)" />
      <!-- Right Vertical Slit Pupil -->
      <polygon points="90,-42 82,-40 80,-8 88,-10" fill="#04070d" />

      <!-- ==================================================================== -->
      <!-- 5. PRECISION NEEDLE RETICLE (Warm Amber Geometric Beak)              -->
      <!-- ==================================================================== -->

      <!-- Upper Beak Bridge -->
      <polygon points="0,-40 -20,20 0,65" fill="#334155" />
      <polygon points="0,-40 20,20 0,65" fill="#475569" />

      <!-- Lower Beak Spear / Needle Tip -->
      <polygon points="0,65 -22,75 0,165" fill="#f59e0b" />
      <polygon points="0,65 22,75 0,165" fill="url(#amber-reticle)" />
      <polygon points="0,165 0,65 4,145" fill="#fbbf24" opacity="0.6" />

      <!-- ==================================================================== -->
      <!-- 6. LOWER THROAT & PLUMAGE FOUNDATION                                -->
      <!-- ==================================================================== -->
      <polygon points="-85,235 -155,315 -60,285" fill="#0f172a" />
      <polygon points="85,235 155,315 60,285" fill="#1e293b" />
      <polygon points="-60,285 -155,315 0,340" fill="#0b0f19" />
      <polygon points="60,285 155,315 0,340" fill="#141c2b" />
      <polygon points="0,165 -85,235 -60,285 0,260" fill="#0f172a" />
      <polygon points="0,165 85,235 60,285 0,260" fill="#1e293b" />

    </g>
  </g>
</svg>`;
}

async function main() {
  const outputDir = path.resolve(__dirname, '../docs/images');
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }

  const svg = buildLogoSvg();
  const svgPath = path.join(outputDir, 'logo.svg');
  const pngPath = path.join(outputDir, 'logo.png');

  fs.writeFileSync(svgPath, svg, 'utf8');

  const resvg = new Resvg(svg, {
    fitTo: { mode: 'width', value: 1024 }
  });
  const pngData = resvg.render().asPng();
  fs.writeFileSync(pngPath, pngData);

  console.log('✓ Successfully generated logo.svg and rendered logo.png at 1024x1024!');
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
