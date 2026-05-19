/* eslint-disable @typescript-eslint/no-explicit-any */
"use client";

import React, { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { motion, AnimatePresence, useMotionValueEvent, useScroll } from "framer-motion";
import Link from "next/link";

// Clean, extensible configuration dictionary for layout contexts
const NAV_CONTEXTS: any = {
  "/simulate": {
    label: "Task A · User Modeling",
    className:
      "text-amber-600 bg-amber-50/60 border-amber-100/80 token-amber",
    accentColor: "bg-amber-500",
  },
  "/recommend": {
    label: "Task B · Recommendations",
    className:
      "text-emerald-700 bg-emerald-50/60 border-emerald-100/80 token-emerald",
    accentColor: "bg-emerald-500",
  },
  "/": {
    label: "DSN × BCT Hackathon 3.0",
    className: "text-[#666] bg-[#f8f8f7] border-[#e8e7e3]",
    accentColor: "bg-emerald-400",
  },
};

const Navbar = () => {
  const pathname = usePathname();
  const currentContext = NAV_CONTEXTS[pathname] || NAV_CONTEXTS["/"];

  // 🔥 scroll state
  const { scrollY } = useScroll();
  const [hidden, setHidden] = useState(false);

  useMotionValueEvent(scrollY, "change", (latest) => {
    const previous = scrollY.getPrevious() ?? 0;

    if (latest > previous && latest > 80) {
      setHidden(true); // scrolling down
    } else {
      setHidden(false); // scrolling up
    }
  });

  return (
    <AnimatePresence mode="wait">
      {!hidden && (
        <motion.nav
          key="navbar"
          initial={{ y: -20, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={{ y: -20, opacity: 0 }}
          transition={{ duration: 0.25, ease: [0.23, 1, 0.32, 1] }}
          className="md:sticky max-w-5xl max-md:px-8 max-md:py-3 md:mx-auto md:rounded-full max-md:border-b  md:mt-10 md:bg-stone-200  inset-4 md:top-0 z-50 md:flex md:items-center md:justify-between md:pl-8 md:pr-4  md:h-16 border-[#E5E4DF] select-none"
        >
          <div className="flex items-center gap-2.5 font-medium tracking-tight">
            <Link href={"/"} className="text-lg font-semibold text-stone-800 black">
              Ningen
            </Link>
            <span className="text-[#c1c0ba] font-light text-xs">/</span>

            <div className="flex items-center gap-2 text-xs text-[#666] font-normal">
              <motion.span
                layout
                className={`w-1.5 h-1.5 rounded-full ${currentContext.accentColor}`}
                transition={{ type: "spring", stiffness: 300, damping: 30 }}
              />
              <span className="text-[#888]">workspace</span>
            </div>
          </div>

          <div className="relative flex items-center h-full max-w-[300px] w-fit min-w-[280px] justify-end max-md:hidden">
            <AnimatePresence mode="wait">
              <motion.div
                key={pathname}
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -4 }}
                transition={{
                  duration: 0.15,
                  ease: [0.23, 1, 0.32, 1],
                }}
                className="absolute right-0"
              >
                <div
                  className={`flex items-center gap-3 border rounded-full px-3 py-2 text-[11px] font-medium uppercase tracking-widest transition-colors duration-200 ${currentContext.className}`}
                >
                  {currentContext.label}
                </div>
              </motion.div>
            </AnimatePresence>
          </div>
        </motion.nav>
      )}
    </AnimatePresence>
  );
};

export default Navbar;