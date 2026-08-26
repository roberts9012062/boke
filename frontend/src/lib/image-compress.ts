// src/lib/image-compress.ts
// 前端图片压缩（canvas 重绘为 webp，长边 ≤1600、质量逐步降至 ≤1MB）。
// 从 compose/image-uploader 提取共享（编辑器弹窗批量上传同规则；DRY）。
// 说明：服务端（CF图床插件）还有一道按设置的压缩兜底，两层幂等互不冲突。

// 压缩后目标大小（1MB）。
const TARGET_SIZE = 1 << 20;

// compressImage 压缩单张图片：小文件原样返回，大文件 canvas 重绘压缩。
// 返回：压缩后的 File（webp 或原文件）。
export async function compressImage(file: File): Promise<File> {
  // 已是小文件直接返回
  if (file.size <= TARGET_SIZE) {
    return file;
  }
  const bitmap = await createImageBitmap(file);
  // 等比缩放：长边不超过 1600px
  const maxSide = 1600;
  let { width, height } = bitmap;
  if (width > maxSide || height > maxSide) {
    const ratio = Math.min(maxSide / width, maxSide / height);
    width = Math.round(width * ratio);
    height = Math.round(height * ratio);
  }
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    return file;
  }
  ctx.drawImage(bitmap, 0, 0, width, height);
  bitmap.close();

  // 质量 0.8 起，逐级降低直到达标（webp 压缩率高）
  const type = "image/webp";
  let quality = 0.8;
  let blob: Blob | null = null;
  for (let i = 0; i < 5; i++) {
    blob = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, type, quality),
    );
    if (blob && blob.size <= TARGET_SIZE) {
      break;
    }
    quality -= 0.15;
  }
  if (!blob) {
    return file;
  }
  return new File([blob], file.name.replace(/\.\w+$/, ".webp"), { type });
}

// readFileAsBase64 文件读为 base64（CF图床直传通道用；不含 data: 前缀）。
export function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result).split(",")[1] || "");
    reader.onerror = () => reject(new Error("读取失败：" + file.name));
    reader.readAsDataURL(file);
  });
}
