import { Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-loader',
	templateUrl: './loader.component.html',
	styleUrls: ['./loader.component.scss']
})
export class LoaderComponent implements OnInit {
	@Input() isTransparent: boolean = false;
	@Input() position: 'absolute' | 'fixed' = 'absolute';

	constructor() {}

	ngOnInit(): void {}
}
