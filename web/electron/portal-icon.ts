import { deflateSync } from "node:zlib";

export function createPortalIconPNG(size = 32): Buffer {
  if (!Number.isInteger(size) || size < 16 || size > 256) throw new Error("Die Portal-Icongröße ist ungültig.");
  const stride = size * 4 + 1;
  const pixels = Buffer.alloc(stride * size);
  const center = (size - 1) / 2;
  const scale = size / 32;
  for (let y = 0; y < size; y++) {
    pixels[y * stride] = 0;
    for (let x = 0; x < size; x++) {
      const offset = y * stride + 1 + x * 4;
      const dx = (x - center) / scale;
      const dy = (y - center) / scale;
      const radius = Math.hypot(dx, dy);
      let rgba: readonly [number, number, number, number] = [0, 0, 0, 0];
      if (radius <= 13.5 && radius >= 9.2) {
        const heat = Math.max(0, Math.min(1, (13.5 - radius) / 4.3));
        rgba = [Math.round(214 + 27 * heat), Math.round(132 + 42 * heat), Math.round(48 + 26 * heat), 255];
      } else if (radius < 9.2) {
        const glow = Math.max(0, 1 - radius / 9.2);
        rgba = [Math.round(39 + 113 * glow), Math.round(18 + 35 * glow), Math.round(22 + 12 * glow), 255];
      }
      const cardinal = (Math.abs(dx) < 1.15 && Math.abs(dy) > 9 && Math.abs(dy) < 15) || (Math.abs(dy) < 1.15 && Math.abs(dx) > 9 && Math.abs(dx) < 15);
      if (cardinal) rgba = [242, 195, 104, 255];
      pixels[offset] = rgba[0]; pixels[offset + 1] = rgba[1]; pixels[offset + 2] = rgba[2]; pixels[offset + 3] = rgba[3];
    }
  }
  const header = Buffer.alloc(13);
  header.writeUInt32BE(size, 0); header.writeUInt32BE(size, 4);
  header[8] = 8; header[9] = 6; header[10] = 0; header[11] = 0; header[12] = 0;
  return Buffer.concat([Buffer.from("89504e470d0a1a0a", "hex"), pngChunk("IHDR", header), pngChunk("IDAT", deflateSync(pixels)), pngChunk("IEND", Buffer.alloc(0))]);
}

function pngChunk(type: string, data: Buffer): Buffer {
  const name = Buffer.from(type, "ascii");
  const chunk = Buffer.alloc(12 + data.length);
  chunk.writeUInt32BE(data.length, 0);
  name.copy(chunk, 4);
  data.copy(chunk, 8);
  chunk.writeUInt32BE(crc32(Buffer.concat([name, data])), 8 + data.length);
  return chunk;
}

function crc32(data: Buffer): number {
  let crc = 0xffffffff;
  for (const byte of data) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
  }
  return (crc ^ 0xffffffff) >>> 0;
}
