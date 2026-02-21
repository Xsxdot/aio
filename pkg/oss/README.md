# OSS 服务使用说明

## 图片预览 URL 优化

为了节省流量和提升加载速度，OSS 服务提供了多种获取优化图片预览 URL 的方法。

### 方法列表

#### 1. GetPreviewUrl - 完整参数版本

支持所有图片处理参数的完整版本：

```go
opts := &oss.ImageProcessOptions{
    Width:   800,           // 宽度（像素）
    Height:  600,           // 高度（像素）
    Quality: 80,            // 质量（1-100）
    Format:  "webp",        // 格式转换（jpg, png, webp等）
    Mode:    "lfit",        // 缩放模式
}
url, err := ossService.GetPreviewUrl(ctx, "path/to/image.jpg", opts, 1*time.Hour)
```

**缩放模式说明：**
- `lfit`（默认）：等比缩放，限制在指定的宽高矩形内，不会变形
- `mfit`：等比缩放，延伸出指定宽高矩形外，填满
- `fill`：固定宽高，自动裁剪
- `pad`：等比缩放，缩放后居中，边缘填充
- `fixed`：固定宽高，强制缩放（可能变形）

#### 2. GetPreviewUrlSimple - 简化版本

只指定宽高，使用默认的等比缩放模式：

```go
// 获取 800x600 的预览图，使用默认质量
url, err := ossService.GetPreviewUrlSimple(ctx, "path/to/image.jpg", 800, 600, 1*time.Hour)
```

#### 3. GetPreviewUrlWithQuality - 指定质量

在调整尺寸的同时指定压缩质量：

```go
// 获取 800x600 的预览图，质量为 80%
url, err := ossService.GetPreviewUrlWithQuality(ctx, "path/to/image.jpg", 800, 600, 80, 1*time.Hour)
```

#### 4. GetPreviewUrlWebP - WebP 格式

转换为 WebP 格式，通常能获得更好的压缩率：

```go
// 获取 800x600 的 WebP 格式预览图，质量为 75%
url, err := ossService.GetPreviewUrlWebP(ctx, "path/to/image.jpg", 800, 600, 75, 1*time.Hour)
```

#### 5. 获取原图

如果不需要处理，传入 `nil` 作为选项参数：

```go
// 获取原图 URL
url, err := ossService.GetPreviewUrl(ctx, "path/to/image.jpg", nil, 1*time.Hour)
```

### 使用场景建议

#### 列表页缩略图
```go
// 小尺寸、高压缩率、WebP 格式
url, err := ossService.GetPreviewUrlWebP(ctx, imageKey, 200, 200, 70, 24*time.Hour)
```

#### 详情页预览图
```go
// 中等尺寸、平衡质量
url, err := ossService.GetPreviewUrlWithQuality(ctx, imageKey, 800, 600, 85, 24*time.Hour)
```

#### 高清预览
```go
// 大尺寸、高质量
opts := &oss.ImageProcessOptions{
    Width:   1920,
    Height:  1080,
    Quality: 90,
    Mode:    "lfit",
}
url, err := ossService.GetPreviewUrl(ctx, imageKey, opts, 24*time.Hour)
```

#### 移动端适配
```go
// 根据设备特性选择格式
format := "jpg"
if supportsWebP {
    format = "webp"
}

opts := &oss.ImageProcessOptions{
    Width:   750,  // 移动端常用宽度
    Height:  0,    // 高度自适应
    Quality: 80,
    Format:  format,
    Mode:    "lfit",
}
url, err := ossService.GetPreviewUrl(ctx, imageKey, opts, 24*time.Hour)
```

### 性能优化建议

1. **合理设置尺寸**：根据实际显示需求设置宽高，避免加载过大的图片
2. **使用 WebP 格式**：在支持的浏览器中使用 WebP 可节省 30-50% 的流量
3. **适当降低质量**：质量 75-85 通常能在视觉效果和文件大小间取得良好平衡
4. **设置合理的过期时间**：常用图片可设置较长的过期时间（如 24 小时），配合 CDN 缓存使用

### 注意事项

- 图片处理参数仅对图片文件有效，视频等其他文件类型会被忽略
- 宽度或高度设置为 0 表示该维度不限制，自动按比例计算
- 质量参数范围为 1-100，建议值为 70-90
- URL 过期时间建议根据业务场景设置，默认最小为 5 秒

### 技术细节

#### 图片处理参数格式

阿里云 OSS 图片处理参数格式为：`image/操作1,参数/操作2,参数/操作3,参数`

例如，当设置 `Width=800, Height=600, Quality=80, Format=webp` 时，生成的参数为：

```
image/resize,m_lfit,w_800,h_600/quality,q_80/format,webp
```

所有图片处理操作都在一个 `image/` 前缀下，用 `/` 分隔不同的操作，每个操作内的参数用 `,` 分隔。


