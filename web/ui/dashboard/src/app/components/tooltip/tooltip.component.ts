
import { Component, Input, OnInit } from '@angular/core';

@Component({
    selector: 'convoy-tooltip',
    imports: [],
    templateUrl: './tooltip.component.html',
    styleUrls: ['./tooltip.component.scss']
})
export class TooltipComponent implements OnInit {
	@Input('size') size: 'sm' | 'md' = 'md';
	@Input('position') position: 'left' | 'right' | 'bottom' | 'top' | 'top-right' | 'top-left' = 'top-left';
	@Input('img') img!: string;
	@Input('color') color: 'primary' | 'white' = 'white';
	@Input('withIcon') withIcon = true;
	@Input('tooltipContent') tooltipContent!: string;
	@Input('type') type: 'primary' | 'white' = 'white';
	@Input('className') class!: string;

	// The tooltip body only appears on hover and focus, so a value whose whole
	// explanation lives in there is unreadable to a screen reader and on touch.
	// Callers whose toggle is not self-explanatory, a bare dash standing in for a
	// value that could not be read, pass the same text here. Left unset the
	// button keeps the name the browser computes from its contents, so existing
	// callers are unchanged.
	@Input('ariaLabel') ariaLabel = '';

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const colors = {
			primary: 'bg-primary-100 after:border-primary-100 text-white-100',
			white: 'shadow-[0px_20px_25px_-5px_rgba(51,65,85,0.1),0px_10px_10px_-5px_rgba(51,65,85,0.04)] bg-white-100 text-black after:border-white-100'
		};
		const positions = {
			bottom: `left-1/2 -translate-x-1/2 after:left-1/2 after:-translate-x-1/2 top-[calc(100%+20px)] after:-top-[19px] after:border-t-transparent after:border-x-transparent`,
			right: `left-[calc(100%+20px)] -top-[100%] after:-left-[20px] after:top-[10px] after:border-l-transparent after:border-y-transparent`,
			left: `right-[calc(100%+20px)] -top-[100%] after:-right-[20px] after:top-[10px] after:border-r-transparent after:border-y-transparent`,
			top: `left-1/2 -translate-x-1/2 after:left-1/2 after:-translate-x-1/2 bottom-[calc(100%+20px)] after:-bottom-[19px] after:border-b-transparent after:border-x-transparent`,
			'top-right': `-right-[160px] after:right-[157px] bottom-[calc(100%+20px)] after:-bottom-[19px] after:border-b-transparent after:border-x-transparent`,
			'top-left': `-right-[16px] after:right-[15px] bottom-[calc(100%+20px)] after:-bottom-[19px] after:border-b-transparent after:border-x-transparent`
		};
		// Overlay classes live on the same [class] binding as position/color.
		// A static class= plus [class] string overwrites the static list, which
		// dropped `absolute` and let min-w-[192px] bodies sit in layout.
		return `absolute opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 group-focus:opacity-100 group-hover:pointer-events-auto group-focus-within:pointer-events-auto group-focus:pointer-events-auto pointer-events-none transition-all z-50 rounded-8px p-14px text-12 text-left after:content-[''] after:absolute font-light after:border-[10px] ${positions[this.position]} ${colors[this.color]} min-w-[192px] ${this.class}`;
	}
}
