import { Component, Input, OnInit } from '@angular/core';

@Component({
    selector: 'convoy-loader, [convoy-loader]',
    templateUrl: './loader.component.html',
    styleUrls: ['./loader.component.scss'],
    standalone: false
})
export class LoaderComponent implements OnInit {
	@Input() isTransparent: boolean = false;
	@Input() position: 'absolute' | 'fixed' | 'relative' = 'absolute';

	constructor() {}

	ngOnInit(): void {}
}
