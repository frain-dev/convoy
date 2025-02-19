import { cn } from '@/lib/utils';

type ConvoyLoaderProps = {
	isTransparent: boolean;
	position: 'absolute' | 'fixed' | 'relative';
};
export function ConvoyLoader(props: ConvoyLoaderProps) {
	const { isTransparent, position = 'absolute' } = props;

	return (
		<div
			className={cn(
				'left-0 right-0 top-0 bottom-0 flex items-center justify-center bg-gradient-radial rounded-8px h-full z-[1]',
				position,
				isTransparent ? 'opacity-50' : '',
			)}
		>
			<img
				src="/assets/img/page-loader.gif"
				alt="loader"
				className="w-150px min-h-150px"
			/>
		</div>
	);
}
