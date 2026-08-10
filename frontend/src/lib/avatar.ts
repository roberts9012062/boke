// src/lib/avatar.ts
// 头像处理工具（M1.7）：头像图片中心裁剪为正方形并压缩（canvas）。
// 设计稿：头像建议正方形，不超过 5MB；此处统一缩至 256px JPEG，控制体积。
"use client";

// cropSquare 将头像图片中心裁剪为正方形并缩放压缩。
// 参数：file 原图；size 目标边长（默认 256px）。
// 返回：压缩后的 File（jpeg），可直接上传 /media。
export async function cropSquare(file: File, size = 256): Promise<File> {
  // 读取为图片（Image 对象绘制到 canvas）
  const bitmap = await createImageBitmap(file);
  // 中心裁剪正方形：取短边为边长，居中截取
  const side = Math.min(bitmap.width, bitmap.height);
  const sx = Math.floor((bitmap.width - side) / 2);
  const sy = Math.floor((bitmap.height - side) / 2);

  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    throw new Error("当前浏览器不支持图片处理");
  }
  // 绘制裁剪区域（圆角裁剪交给 CSS，此处仅方形）
  ctx.drawImage(bitmap, sx, sy, side, side, 0, 0, size, size);
  bitmap.close();

  // canvas 转 Blob（JPEG 0.85 质量，体积远小于 5MB 上限）
  const blob = await new Promise<Blob | null>((resolve) => {
    canvas.toBlob(resolve, "image/jpeg", 0.85);
  });
  if (!blob) {
    throw new Error("图片压缩失败");
  }
  // 文件名沿用原扩展名（.jpg），类型固定 jpeg
  return new File([blob], "avatar.jpg", { type: "image/jpeg" });
}
