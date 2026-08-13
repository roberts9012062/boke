// src/lib/avatar.ts
// 头像处理工具（M1.7）：头像图片中心裁剪为正方形并压缩（canvas）。
// 设计稿：头像建议正方形，不超过 5MB；此处统一缩至 256px JPEG，控制体积。
"use client";

// CropRegion 圆形头像裁剪参数（由裁剪器视口坐标反解原图区域）。
export interface CropRegion {
  scale: number; // 图片显示缩放倍数
  offsetX: number; // 图片左上角相对视口的水平位移（px）
  offsetY: number; // 图片左上角相对视口的垂直位移（px）
  viewport: number; // 裁剪视口边长（px，正方形）
  outputSize: number; // 输出头像边长（px）
}

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

// cropImageRegion 按裁剪器视口坐标裁剪头像（圆形区域的内切正方形）。
// 参数：file 原图；region 裁剪参数（scale/offset/viewport 等）。
// 说明：视口为边长 viewport 的正方形、圆心居中。图片经 scale + offset 变换后，
//       反解圆心在原图坐标 (cx,cy) 与裁剪边长 viewport/scale，输出内切正方形。
// 返回：裁剪压缩后的 File（jpeg），可直接上传 /media。
export async function cropImageRegion(file: File, region: CropRegion): Promise<File> {
  const bitmap = await createImageBitmap(file);
  const { scale, offsetX, offsetY, viewport, outputSize } = region;

  // 反解原图内切正方形区域：图片层 transform 为 translate(offset) scale(s)，原点在左上角
  // 视口中心 (viewport/2) 映射回原图坐标 (viewport/2 - offset)/scale
  const side = viewport / scale;
  // 越界兜底（浮点误差防护，正常交互下已在边界内）
  let sx = Math.max(0, Math.min(-offsetX / scale, bitmap.width - side));
  let sy = Math.max(0, Math.min(-offsetY / scale, bitmap.height - side));
  const cropSide = Math.min(side, bitmap.width - sx, bitmap.height - sy);
  if (cropSide <= 0) {
    bitmap.close();
    throw new Error("裁剪区域无效，请重新选择图片");
  }

  const canvas = document.createElement("canvas");
  canvas.width = outputSize;
  canvas.height = outputSize;
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    bitmap.close();
    throw new Error("当前浏览器不支持图片处理");
  }
  // 圆形头像显示为「内切正方形 + CSS 圆角」，此处输出方形即可
  ctx.drawImage(bitmap, sx, sy, cropSide, cropSide, 0, 0, outputSize, outputSize);
  bitmap.close();

  // canvas 转 Blob（JPEG 0.9 质量：头像体积小，取更高清晰度）
  const blob = await new Promise<Blob | null>((resolve) => {
    canvas.toBlob(resolve, "image/jpeg", 0.9);
  });
  if (!blob) {
    throw new Error("图片压缩失败");
  }
  return new File([blob], "avatar.jpg", { type: "image/jpeg" });
}
