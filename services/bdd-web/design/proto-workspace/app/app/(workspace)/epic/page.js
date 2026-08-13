"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

// Epics died as a concept (steering: "get rid of epics section at all" —
// stories are now flat, direct children of Product). /epic has no file to
// show anymore, but old links/bookmarks to it should still land somewhere
// live rather than 404 — bounce straight to /product.
export default function EpicPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace("/product");
  }, [router]);

  return null;
}
