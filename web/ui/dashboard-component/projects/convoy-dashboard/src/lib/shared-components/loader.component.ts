import { ChangeDetectionStrategy, Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-loader',
	changeDetection: ChangeDetectionStrategy.OnPush,
	template: `
		<div [class]="'loader ' + (isTransparent ? 'transparent' : '')">
			<img src="/assets/img/page-loader.gif" alt="loader" />
		</div>
	`,
	styles: [
		`
			.loader {
				position: absolute;
				left: 0;
				right: 0;
				top: 0;
				bottom: 0;
				display: flex;
				justify-content: center;
				align-items: center;
				background: #fff;
				height: 100%;
				border-radius: 8px;
				z-index: 1;
			}

			.loader img {
				width: 20%;
			}

			.loader.transparent {
				opacity: 0.5;
			}
		`
	]
})
export class ConvoyLoaderComponent implements OnInit {
	constructor() {}
	@Input('isTransparent') isTransparent: boolean = false;

	async ngOnInit() {}
}
