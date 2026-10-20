package com.lossalt.moyue;

import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Matrix;
import android.media.ExifInterface;

import java.io.ByteArrayInputStream;

/** Bitmap decoding with sampling, EXIF rotation and a width cap. */
final class ImageUtil {
    private ImageUtil() {}

    static Bitmap decode(byte[] data, int maxWidth) {
        if (data == null || data.length == 0) return null;
        try {
            BitmapFactory.Options bounds = new BitmapFactory.Options();
            bounds.inJustDecodeBounds = true;
            BitmapFactory.decodeByteArray(data, 0, data.length, bounds);
            if (bounds.outWidth <= 0 || bounds.outHeight <= 0) return null;

            int sample = 1;
            int target = Math.max(1, maxWidth);
            while (bounds.outWidth / (sample * 2) >= target && sample < 32) {
                sample *= 2;
            }
            BitmapFactory.Options opts = new BitmapFactory.Options();
            opts.inSampleSize = sample;
            opts.inPreferredConfig = Bitmap.Config.ARGB_8888;
            Bitmap bitmap = BitmapFactory.decodeByteArray(data, 0, data.length, opts);
            if (bitmap == null) return null;

            int orientation = exifOrientation(data);
            if (orientation != ExifInterface.ORIENTATION_NORMAL) {
                Matrix m = new Matrix();
                switch (orientation) {
                    case ExifInterface.ORIENTATION_ROTATE_90: m.postRotate(90); break;
                    case ExifInterface.ORIENTATION_ROTATE_180: m.postRotate(180); break;
                    case ExifInterface.ORIENTATION_ROTATE_270: m.postRotate(270); break;
                    case ExifInterface.ORIENTATION_FLIP_HORIZONTAL: m.postScale(-1, 1); break;
                    case ExifInterface.ORIENTATION_FLIP_VERTICAL: m.postScale(1, -1); break;
                    case ExifInterface.ORIENTATION_TRANSPOSE:
                        m.postRotate(90); m.postScale(-1, 1); break;
                    case ExifInterface.ORIENTATION_TRANSVERSE:
                        m.postRotate(270); m.postScale(-1, 1); break;
                    default: break;
                }
                Bitmap rotated = Bitmap.createBitmap(bitmap, 0, 0, bitmap.getWidth(), bitmap.getHeight(), m, true);
                if (rotated != bitmap) {
                    bitmap.recycle();
                    bitmap = rotated;
                }
            }
            if (maxWidth > 0 && bitmap.getWidth() > maxWidth) {
                int h = Math.max(1, Math.round(bitmap.getHeight() * (maxWidth / (float) bitmap.getWidth())));
                Bitmap scaled = Bitmap.createScaledBitmap(bitmap, maxWidth, h, true);
                if (scaled != bitmap) {
                    bitmap.recycle();
                    bitmap = scaled;
                }
            }
            return bitmap;
        } catch (Exception e) {
            return null;
        } catch (OutOfMemoryError e) {
            return null;
        }
    }

    private static int exifOrientation(byte[] data) {
        try {
            ExifInterface exif = new ExifInterface(new ByteArrayInputStream(data));
            return exif.getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL);
        } catch (Exception e) {
            return ExifInterface.ORIENTATION_NORMAL;
        }
    }
}
