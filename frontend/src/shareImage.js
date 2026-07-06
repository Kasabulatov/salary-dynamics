// Share-image export: composites the verdict + the rendered Recharts SVG
// onto a canvas and downloads a watermarked PNG. Zero dependencies — the
// chart is already SVG, so we serialize and rasterize it ourselves.

const W = 1200
const H = 630
const SITE = 'salary-dynamics.vercel.app'

function wrapText(ctx, text, maxWidth) {
  const words = text.split(' ')
  const lines = []
  let line = ''
  for (const word of words) {
    const attempt = line ? line + ' ' + word : word
    if (ctx.measureText(attempt).width > maxWidth && line) {
      lines.push(line)
      line = word
    } else {
      line = attempt
    }
  }
  if (line) lines.push(line)
  return lines
}

async function svgToImage(svgEl) {
  const clone = svgEl.cloneNode(true)
  // Ensure explicit dimensions for rasterization.
  const rect = svgEl.getBoundingClientRect()
  clone.setAttribute('width', rect.width)
  clone.setAttribute('height', rect.height)
  clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg')
  const xml = new XMLSerializer().serializeToString(clone)
  const src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(xml)
  const img = new Image()
  await new Promise((resolve, reject) => {
    img.onload = resolve
    img.onerror = () => reject(new Error('chart rasterization failed'))
    img.src = src
  })
  return { img, width: rect.width, height: rect.height }
}

// createShareImageBlob renders the result to a 1200x630 PNG blob
// (social-card size). chartContainer is the DOM node wrapping the chart.
export async function createShareImageBlob({ chartContainer, verdict }) {
  const canvas = document.createElement('canvas')
  canvas.width = W
  canvas.height = H
  const ctx = canvas.getContext('2d')

  // Background + header.
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, W, H)
  ctx.fillStyle = '#6e6e73'
  ctx.font = '600 22px sans-serif'
  ctx.fillText('Offer Comparison', 60, 64)

  // Verdict, wrapped.
  ctx.fillStyle = '#1d1d1f'
  ctx.font = '700 34px sans-serif'
  const lines = wrapText(ctx, verdict, W - 120)
  let y = 118
  for (const line of lines.slice(0, 4)) {
    ctx.fillText(line, 60, y)
    y += 44
  }

  // Chart, scaled to fit the remaining area.
  const svg = chartContainer?.querySelector('svg')
  if (svg) {
    try {
      const { img, width, height } = await svgToImage(svg)
      const areaTop = y + 16
      const areaHeight = H - areaTop - 70
      const scale = Math.min((W - 120) / width, areaHeight / height)
      const drawW = width * scale
      const drawH = height * scale
      ctx.drawImage(img, (W - drawW) / 2, areaTop, drawW, drawH)
    } catch {
      // Chart rasterization is best-effort; the verdict alone still shares.
    }
  }

  // Watermark: shared images drive traffic back.
  ctx.fillStyle = '#86868b'
  ctx.font = '500 20px sans-serif'
  const mark = `${SITE} · what is your salary really worth?`
  ctx.fillText(mark, W - 60 - ctx.measureText(mark).width, H - 36)

  return new Promise((resolve) => canvas.toBlob(resolve, 'image/png'))
}

// shareResult: native share sheet with the image attached (Telegram /
// WhatsApp / LinkedIn appear as targets on devices that have them);
// falls back to downloading the PNG where Web Share can't send files.
export async function shareResult({ chartContainer, verdict }) {
  const blob = await createShareImageBlob({ chartContainer, verdict })
  const file = new File([blob], 'offer-comparison.png', { type: 'image/png' })

  if (navigator.canShare?.({ files: [file] })) {
    try {
      await navigator.share({
        files: [file],
        text: `${verdict} — via https://${SITE}/compare`,
      })
      return 'shared'
    } catch (e) {
      if (e.name === 'AbortError') return 'cancelled' // user closed the sheet
      // fall through to download on real failures
    }
  }
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'offer-comparison.png'
  a.click()
  URL.revokeObjectURL(url)
  return 'downloaded'
}

// Text+link share URLs (these endpoints cannot carry an image — the image
// path is the native sheet or the downloaded PNG).
export function shareLinks(verdict) {
  const link = `https://${SITE}/compare`
  const text = encodeURIComponent(`${verdict} — see for yourself:`)
  return {
    telegram: `https://t.me/share/url?url=${encodeURIComponent(link)}&text=${text}`,
    whatsapp: `https://wa.me/?text=${text}%20${encodeURIComponent(link)}`,
    linkedin: `https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(link)}`,
  }
}
