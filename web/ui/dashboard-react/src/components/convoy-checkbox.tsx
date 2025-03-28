import { cn } from '@/lib/utils';
import { type ChangeEventHandler } from 'react';

/**
 * Use as a child of <FormControl> in react-hook-form
 */
export function ConvoyCheckbox(props: {
	label: string
	isChecked: boolean;
	onChange: ChangeEventHandler<HTMLInputElement>;
	className?: string;
}) {
	return (
		<label className="flex items-center gap-2 hover:cursor-pointer">
			<div className="relative">
				<input
					type="checkbox"
					className={cn(
						'peer appearance-none w-[14px] h-[14px] border-[1px] border-new.primary-300 rounded-sm bg-white-100 mt-1 shrink-0 checked:bg-new.primary-300 checked:border-0 cursor-pointer',
						props.className && props.className,
					)}
					defaultChecked={props.isChecked}
					onChange={props.onChange}
				/>
				<svg
					className={cn(
						'absolute w-3 h-3 mt-1 hidden peer-checked:block top-[0.5px] right-[1px]',
						props.className && props.className
					)}
					xmlns="http://www.w3.org/2000/svg"
					viewBox="0 0 24 24"
					fill="none"
					stroke="white"
					strokeWidth="4"
					strokeLinecap="round"
					strokeLinejoin="round"
				>
					<polyline points="20 6 9 17 4 12"></polyline>
				</svg>
			</div>

			<span className="block text-neutral-9 text-xs">{props.label}</span>
		</label>
	);
}
