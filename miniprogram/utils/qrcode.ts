const MODULE_COUNT = 33;
const TOTAL_DATA_COUNT = 80;
const ERROR_CORRECTION_COUNT = 20;
const MODE_8BIT_BYTE = 4;
const ERROR_CORRECT_LEVEL_L = 1;
const PAD0 = 0xec;
const PAD1 = 0x11;
const G15 = 1335;
const G15_MASK = 21522;
const POSITION_ADJUST_PATTERN = [6, 26];

const EXP_TABLE: number[] = new Array(256);
const LOG_TABLE: number[] = new Array(256);

for (let i = 0; i < 8; i++) {
  EXP_TABLE[i] = 1 << i;
}
for (let i = 8; i < 256; i++) {
  EXP_TABLE[i] = EXP_TABLE[i - 4] ^ EXP_TABLE[i - 5] ^ EXP_TABLE[i - 6] ^ EXP_TABLE[i - 8];
}
for (let i = 0; i < 255; i++) {
  LOG_TABLE[EXP_TABLE[i]] = i;
}

function createPolynomial(num: number[], shift: number) {
  let offset = 0;
  while (offset < num.length && num[offset] === 0) {
    offset++;
  }

  const result = new Array(num.length - offset + shift);
  for (let i = 0; i < num.length - offset; i++) {
    result[i] = num[i + offset];
  }
  for (let i = num.length - offset; i < result.length; i++) {
    result[i] = 0;
  }

  return {
    num: result
  };
}

function polyGet(polynomial: any, index: number): number {
  return polynomial.num[index];
}

function polyGetLength(polynomial: any): number {
  return polynomial.num.length;
}

function polyMultiply(left: any, right: any) {
  const result = new Array(polyGetLength(left) + polyGetLength(right) - 1);
  for (let i = 0; i < result.length; i++) {
    result[i] = 0;
  }

  for (let i = 0; i < polyGetLength(left); i++) {
    for (let j = 0; j < polyGetLength(right); j++) {
      result[i + j] ^= gexp(glog(polyGet(left, i)) + glog(polyGet(right, j)));
    }
  }

  return createPolynomial(result, 0);
}

function polyMod(left: any, right: any): any {
  if (polyGetLength(left) - polyGetLength(right) < 0) {
    return left;
  }

  const ratio = glog(polyGet(left, 0)) - glog(polyGet(right, 0));
  const result = left.num.slice();

  for (let i = 0; i < polyGetLength(right); i++) {
    result[i] ^= gexp(glog(polyGet(right, i)) + ratio);
  }

  return polyMod(createPolynomial(result, 0), right);
}

function createBitBuffer() {
  return {
    buffer: [],
    length: 0
  };
}

function bitBufferPutBit(bitBuffer: any, bit: boolean) {
  const bufIndex = Math.floor(bitBuffer.length / 8);
  if (bitBuffer.buffer.length <= bufIndex) {
    bitBuffer.buffer.push(0);
  }

  if (bit) {
    bitBuffer.buffer[bufIndex] |= 0x80 >>> (bitBuffer.length % 8);
  }

  bitBuffer.length++;
}

function bitBufferPut(bitBuffer: any, num: number, length: number) {
  for (let i = 0; i < length; i++) {
    bitBufferPutBit(bitBuffer, ((num >>> (length - i - 1)) & 1) === 1);
  }
}

function glog(n: number): number {
  if (n < 1) {
    throw new Error('QR log input invalid');
  }
  return LOG_TABLE[n];
}

function gexp(n: number): number {
  while (n < 0) {
    n += 255;
  }
  while (n >= 256) {
    n -= 255;
  }
  return EXP_TABLE[n];
}

function getBCHDigit(data: number): number {
  let digit = 0;
  let value = data;
  while (value !== 0) {
    digit++;
    value >>>= 1;
  }
  return digit;
}

function getBCHTypeInfo(data: number): number {
  let value = data << 10;
  while (getBCHDigit(value) - getBCHDigit(G15) >= 0) {
    value ^= G15 << (getBCHDigit(value) - getBCHDigit(G15));
  }
  return ((data << 10) | value) ^ G15_MASK;
}

function getErrorCorrectPolynomial(errorCorrectLength: number) {
  let polynomial = createPolynomial([1], 0);
  for (let i = 0; i < errorCorrectLength; i++) {
    polynomial = polyMultiply(polynomial, createPolynomial([1, gexp(i)], 0));
  }
  return polynomial;
}

function encodeText(text: string): number[] {
  const bytes: number[] = [];
  for (let i = 0; i < text.length; i++) {
    const code = text.charCodeAt(i);
    if (code > 255) {
      throw new Error('二维码内容仅支持 ASCII 字符');
    }
    bytes.push(code);
  }
  return bytes;
}

function createData(text: string): number[] {
  const dataBytes = encodeText(text);
  if (dataBytes.length > 78) {
    throw new Error('二维码内容过长');
  }

  const bitBuffer = createBitBuffer();
  bitBufferPut(bitBuffer, MODE_8BIT_BYTE, 4);
  bitBufferPut(bitBuffer, dataBytes.length, 8);

  for (let i = 0; i < dataBytes.length; i++) {
    bitBufferPut(bitBuffer, dataBytes[i], 8);
  }

  if (bitBuffer.length + 4 <= TOTAL_DATA_COUNT * 8) {
    bitBufferPut(bitBuffer, 0, 4);
  }

  while (bitBuffer.length % 8 !== 0) {
    bitBufferPutBit(bitBuffer, false);
  }

  let isFirstPad = true;
  while (bitBuffer.buffer.length < TOTAL_DATA_COUNT) {
    bitBuffer.buffer.push(isFirstPad ? PAD0 : PAD1);
    isFirstPad = !isFirstPad;
  }

  const rsPolynomial = getErrorCorrectPolynomial(ERROR_CORRECTION_COUNT);
  const rawPolynomial = createPolynomial(bitBuffer.buffer, polyGetLength(rsPolynomial) - 1);
  const modPolynomial = polyMod(rawPolynomial, rsPolynomial);
  const ecData: number[] = new Array(polyGetLength(rsPolynomial) - 1);

  for (let i = 0; i < ecData.length; i++) {
    const modIndex = i + polyGetLength(modPolynomial) - ecData.length;
    ecData[i] = modIndex >= 0 ? polyGet(modPolynomial, modIndex) : 0;
  }

  return bitBuffer.buffer.concat(ecData);
}

function createModules() {
  const modules: any[] = [];
  for (let row = 0; row < MODULE_COUNT; row++) {
    modules[row] = [];
    for (let col = 0; col < MODULE_COUNT; col++) {
      modules[row][col] = null;
    }
  }
  return modules;
}

function setupPositionProbePattern(modules: any[], row: number, col: number) {
  for (let r = -1; r <= 7; r++) {
    if (row + r <= -1 || MODULE_COUNT <= row + r) continue;

    for (let c = -1; c <= 7; c++) {
      if (col + c <= -1 || MODULE_COUNT <= col + c) continue;

      const isOuter = (r >= 0 && r <= 6 && (c === 0 || c === 6)) || (c >= 0 && c <= 6 && (r === 0 || r === 6));
      const isInner = r >= 2 && r <= 4 && c >= 2 && c <= 4;
      modules[row + r][col + c] = isOuter || isInner;
    }
  }
}

function setupTimingPattern(modules: any[]) {
  for (let i = 8; i < MODULE_COUNT - 8; i++) {
    if (modules[i][6] === null) {
      modules[i][6] = i % 2 === 0;
    }
    if (modules[6][i] === null) {
      modules[6][i] = i % 2 === 0;
    }
  }
}

function setupPositionAdjustPattern(modules: any[]) {
  for (let i = 0; i < POSITION_ADJUST_PATTERN.length; i++) {
    for (let j = 0; j < POSITION_ADJUST_PATTERN.length; j++) {
      const row = POSITION_ADJUST_PATTERN[i];
      const col = POSITION_ADJUST_PATTERN[j];

      if (modules[row][col] !== null) continue;

      for (let r = -2; r <= 2; r++) {
        for (let c = -2; c <= 2; c++) {
          const isOuter = r === -2 || r === 2 || c === -2 || c === 2;
          const isCenter = r === 0 && c === 0;
          modules[row + r][col + c] = isOuter || isCenter;
        }
      }
    }
  }
}

function setupTypeInfo(modules: any[], maskPattern: number) {
  const data = (ERROR_CORRECT_LEVEL_L << 3) | maskPattern;
  const bits = getBCHTypeInfo(data);

  for (let i = 0; i < 15; i++) {
    const mod = ((bits >> i) & 1) === 1;

    if (i < 6) {
      modules[i][8] = mod;
    } else if (i < 8) {
      modules[i + 1][8] = mod;
    } else {
      modules[MODULE_COUNT - 15 + i][8] = mod;
    }

    if (i < 8) {
      modules[8][MODULE_COUNT - i - 1] = mod;
    } else if (i < 9) {
      modules[8][15 - i] = mod;
    } else {
      modules[8][15 - i - 1] = mod;
    }
  }

  modules[MODULE_COUNT - 8][8] = true;
}

function getMask(maskPattern: number, row: number, col: number): boolean {
  if (maskPattern === 0) {
    return (row + col) % 2 === 0;
  }
  return false;
}

function mapData(modules: any[], data: number[], maskPattern: number) {
  let inc = -1;
  let row = MODULE_COUNT - 1;
  let bitIndex = 7;
  let byteIndex = 0;

  for (let col = MODULE_COUNT - 1; col > 0; col -= 2) {
    if (col === 6) {
      col--;
    }

    while (true) {
      for (let c = 0; c < 2; c++) {
        if (modules[row][col - c] !== null) {
          continue;
        }

        let dark = false;
        if (byteIndex < data.length) {
          dark = ((data[byteIndex] >>> bitIndex) & 1) === 1;
        }

        if (getMask(maskPattern, row, col - c)) {
          dark = !dark;
        }

        modules[row][col - c] = dark;
        bitIndex--;

        if (bitIndex === -1) {
          byteIndex++;
          bitIndex = 7;
        }
      }

      row += inc;

      if (row < 0 || MODULE_COUNT <= row) {
        row -= inc;
        inc = -inc;
        break;
      }
    }
  }
}

function createQrModules(text: string) {
  const modules = createModules();
  const data = createData(text);

  setupPositionProbePattern(modules, 0, 0);
  setupPositionProbePattern(modules, MODULE_COUNT - 7, 0);
  setupPositionProbePattern(modules, 0, MODULE_COUNT - 7);
  setupTimingPattern(modules);
  setupPositionAdjustPattern(modules);
  setupTypeInfo(modules, 0);
  mapData(modules, data, 0);

  return modules;
}

export function drawQrCode(canvasId: string, text: string, size: number, page: any): Promise<void> {
  return new Promise((resolve, reject) => {
    try {
      const modules = createQrModules(text);
      const padding = 24;
      const qrSize = size - padding * 2;
      const cellSize = qrSize / MODULE_COUNT;
      const ctx = wx.createCanvasContext(canvasId, page);

      ctx.setFillStyle('#ffffff');
      ctx.fillRect(0, 0, size, size);

      ctx.setFillStyle('#000000');
      for (let row = 0; row < MODULE_COUNT; row++) {
        for (let col = 0; col < MODULE_COUNT; col++) {
          if (!modules[row][col]) continue;
          ctx.fillRect(
            padding + col * cellSize,
            padding + row * cellSize,
            cellSize,
            cellSize
          );
        }
      }

      ctx.draw(false, () => resolve());
    } catch (err) {
      reject(err);
    }
  });
}
