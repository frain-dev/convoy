import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { Router } from '@angular/router';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { PAGINATION } from 'src/app/models/global.model';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TableComponent } from 'src/app/components/table/table.component';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { TagComponent } from 'src/app/components/tag/tag.component';

@Component({
	selector: 'convoy-event-logs',
	standalone: true,
	imports: [CommonModule, CardComponent, ButtonComponent, EmptyStateComponent, TagComponent, TableLoaderModule, TableComponent, TableCellComponent, TableHeadCellComponent, TableHeadComponent, TableRowComponent],
	templateUrl: './event-logs.component.html',
	styleUrls: ['./event-logs.component.scss']
})
export class EventLogsComponent implements OnInit {
	eventLogsTableHead = ['Event Type', 'Endpoint Name', 'Time Created', ''];
	isLoadingEventLogs = false;
	eventLogs!: { pagination: PAGINATION; content: any };

	constructor(public privateService: PrivateService, private router: Router) {}

	ngOnInit(): void {}

	getEvents(requestDetails?: { page?: number }) {}
}
