interface NavBadgeProps {
    count: number
}

export default function NavBadge({ count }: NavBadgeProps) {
    return (
        <span className="flex items-center justify-center min-w-[18px] h-[18px] px-1 rounded-full bg-red-500 text-white text-[10px] font-semibold leading-none">
            {count > 99 ? '99+' : count}
        </span>
    )
}
