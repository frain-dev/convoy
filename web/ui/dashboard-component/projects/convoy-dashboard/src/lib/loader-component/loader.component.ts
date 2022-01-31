import { Component, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-loader',
	template: `
		<div class="loader">
			<img src="/assets/img/loader.gif" alt="loader" />
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
				border-radius: 8px;
				z-index: 1;
				/* opacity: 0; */
			}

			.loader img {
				width: 25%;
			}
		`
	]
})
export class ConvoyLoaderComponent implements OnInit {
	constructor() {}

	async ngOnInit() {}
}
