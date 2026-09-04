import * as React from "react"
import { Input as InputPrimitive } from "@/components/ui/input-primitive"

interface InputProps
  extends Omit<React.ComponentProps<"input">, "onChange" | "value"> {
  label?: string
  value: string | number
  onChange: (value: string) => void
}

export const Input: React.FC<InputProps> = ({
  label,
  value,
  onChange,
  className,
  id,
  ...props
}) => {
  const inputId = id || label?.toLowerCase().replace(/\s+/g, "-")

  return (
    <div className="space-y-1">
      {label && (
        <label
          htmlFor={inputId}
          className="block text-sm font-medium text-foreground"
        >
          {label}
        </label>
      )}
      <InputPrimitive
        id={inputId}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={className}
        {...props}
      />
    </div>
  )
}
