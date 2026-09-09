"use client"

import Image from "next/image"
import { useEffect, useState } from "react"

const showcases = [
  {
    number: "01",
    title: "服务运营总览",
    image: "/images/home/operations-overview.png",
  },
  {
    number: "02",
    title: "工程师处理工作台",
    image: "/images/home/ticket-workbench.png",
  },
  {
    number: "03",
    title: "视频协作与参与记录",
    image: "/images/home/video-session.png",
  },
  {
    number: "04",
    title: "工单处理与责任链",
    image: "/images/home/ticket-lifecycle.png",
  },
  {
    number: "05",
    title: "组织排班与工单分配",
    image: "/images/home/team-dispatch.png",
  },
] as const

const loopedShowcases = [...showcases, showcases[0]]

export function ProductShowcaseScroller() {
  const [slideIndex, setSlideIndex] = useState(0)
  const [activeIndex, setActiveIndex] = useState(0)
  const [transitionEnabled, setTransitionEnabled] = useState(true)

  useEffect(() => {
    const timer = window.setInterval(() => {
      setTransitionEnabled(true)
      setSlideIndex((current) => current + 1)
    }, 4200)

    return () => window.clearInterval(timer)
  }, [])

  const handleTransitionEnd = () => {
    if (slideIndex === showcases.length) {
      setActiveIndex(0)
      setTransitionEnabled(false)
      setSlideIndex(0)
      window.requestAnimationFrame(() => {
        window.requestAnimationFrame(() => setTransitionEnabled(true))
      })
      return
    }

    setActiveIndex(slideIndex)
  }

  return (
    <div className="mx-auto w-full max-w-[1240px] px-4 sm:px-8 lg:px-10">
      <div className="overflow-hidden rounded-md border border-slate-800 bg-[#07131f] shadow-[0_30px_80px_rgba(15,23,42,0.2)]">
        <div className="grid min-h-12 grid-cols-[1fr_auto_1fr] items-center border-b border-white/10 px-4 sm:px-5">
          <span className="flex items-center gap-2" aria-hidden="true"><span className="size-2.5 rounded-full bg-rose-400" /><span className="size-2.5 rounded-full bg-amber-300" /><span className="size-2.5 rounded-full bg-emerald-400" /></span>
          <span className="hidden font-mono text-[10px] text-slate-500 sm:block">enterprise.remotehelpdesk</span>
          <span className="justify-self-end font-mono text-[10px] text-emerald-300">LIVE · {showcases[activeIndex].number} / {String(showcases.length).padStart(2, "0")}</span>
        </div>

        <div className="overflow-hidden" aria-label="产品界面自动轮播" aria-live="off">
          <div
            className={transitionEnabled ? "flex transition-transform duration-700 ease-[cubic-bezier(0.65,0,0.35,1)]" : "flex"}
            style={{ transform: `translate3d(-${slideIndex * 100}%, 0, 0)` }}
            onTransitionEnd={handleTransitionEnd}
          >
            {loopedShowcases.map((showcase, index) => (
              <figure key={`${showcase.number}-${index}`} className="relative aspect-[4/3] w-full shrink-0 overflow-hidden bg-[#eef2f6] sm:aspect-[2/1]">
                <Image src={showcase.image} alt={showcase.title} fill priority={index === 0} unoptimized sizes="(min-width: 1280px) 1160px, calc(100vw - 32px)" className="object-cover object-top sm:object-contain" />
              </figure>
            ))}
          </div>
        </div>

        <div className="flex min-h-14 items-center justify-between gap-5 border-t border-white/10 px-5 sm:px-7">
          <strong className="text-sm font-medium text-white sm:text-base">{showcases[activeIndex].title}</strong>
          <div className="flex w-24 gap-1.5" aria-hidden="true">
            {showcases.map((showcase, index) => <span key={showcase.number} className={`h-0.5 flex-1 transition-colors duration-500 ${activeIndex === index ? "bg-emerald-300" : "bg-white/20"}`} />)}
          </div>
        </div>
      </div>
    </div>
  )
}
