package com.lossalt.moyue;

import java.io.File;

/** JVM-only smoke test; not packaged into the APK. */
public final class ParserSmoke {
    public static void main(String[] args) throws Exception {
        File file = new File(args.length > 0 ? args[0] : "../examples/灯下集.epub");
        Epub.Book book = Epub.parse(file);
        System.out.println("title: " + book.title);
        System.out.println("author: " + book.author);
        System.out.println("chapters: " + book.chapters.size());
        int blocks = 0;
        for (Epub.Chapter c : book.chapters) blocks += c.blocks.size();
        System.out.println("blocks: " + blocks);
        for (int i = 0; i < Math.min(3, book.chapters.size()); i++) {
            Epub.Chapter c = book.chapters.get(i);
            System.out.println("  " + (i + 1) + ". [L" + c.level + "] " + c.title + " (" + c.blocks.size() + " blocks)");
            if (!c.blocks.isEmpty()) System.out.println("     " + c.blocks.get(0).text);
        }
        if (book.chapters.isEmpty() || blocks == 0) throw new IllegalStateException("empty parse");
    }
}
