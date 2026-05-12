import { Component, Input, OnInit } from '@angular/core';


@Component({
    selector: 'convoy-skeleton-loader',
    imports: [],
    templateUrl: './skeleton-loader.component.html',
    styleUrls: ['./skeleton-loader.component.scss']
})
export class SkeletonLoaderComponent implements OnInit {
	@Input('className') class!: string;

	constructor() {}

	ngOnInit(): void {}
}
