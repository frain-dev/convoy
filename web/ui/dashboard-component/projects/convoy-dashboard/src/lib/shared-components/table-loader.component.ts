import { ChangeDetectionStrategy, Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-table-loader',
	changeDetection: ChangeDetectionStrategy.OnPush,
	template: `
		<table class="table">
			<thead>
				<tr class="table--head">
					<th *ngFor="let head of tableHead" scope="col">{{ head }}</th>
				</tr>
			</thead>
			<tbody>
				<tr class="table--date-row">
					<td>
						<div>
							<div class="skeleton-loader smaller padding-top--unset padding-bottom--unset"></div>
						</div>
					</td>
					<td></td>
					<td></td>
					<td></td>
				</tr>
				<tr>
					<td>
						<div>
							<div class="skeleton-loader data"></div>
						</div>
					</td>
					<td *ngFor="let head of tableHead.slice(1)">
						<div class="skeleton-loader data"></div>
					</td>
				</tr>
                <tr>
					<td>
						<div>
							<div class="skeleton-loader data"></div>
						</div>
					</td>
					<td *ngFor="let head of tableHead.slice(1)">
						<div class="skeleton-loader data"></div>
					</td>
				</tr>
                <tr>
					<td>
						<div>
							<div class="skeleton-loader data"></div>
						</div>
					</td>
					<td *ngFor="let head of tableHead.slice(1)">
						<div class="skeleton-loader data"></div>
					</td>
				</tr>
                <tr>
					<td>
						<div>
							<div class="skeleton-loader data"></div>
						</div>
					</td>
					<td *ngFor="let head of tableHead.slice(1)">
						<div class="skeleton-loader data"></div>
					</td>
				</tr>
				
				<tr class="table--date-row">
					<td>
						<div>
							<div class="skeleton-loader smaller padding-top--unset padding-bottom--unset"></div>
						</div>
					</td>
					<td></td>
					<td></td>
					<td></td>
				</tr>
				<tr>
					<td>
						<div>
							<div class="skeleton-loader data"></div>
						</div>
					</td>
					<td *ngFor="let head of tableHead.slice(1)">
						<div class="skeleton-loader data"></div>
					</td>
				</tr>
				<tr>
					<td>
						<div>
							<div class="skeleton-loader data"></div>
						</div>
					</td>
					<td *ngFor="let head of tableHead.slice(1)">
						<div class="skeleton-loader data"></div>
					</td>
				</tr>
			</tbody>
		</table>
	`
})
export class ConvoyTableLoaderComponent implements OnInit {
	constructor() {}
	@Input('tableHead') tableHead!: string[];

	async ngOnInit() {}
}