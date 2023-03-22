import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'convoy-pagination',
	standalone: true,
	imports: [CommonModule, RouterModule, ButtonComponent],
	templateUrl: './pagination.component.html',
	styleUrls: ['./pagination.component.scss']
})
export class PaginationComponent implements OnInit {
	@Input('pagination') paginationData?: PAGINATION;
	@Output('paginate') paginate = new EventEmitter<CURSOR>();

	constructor() {}

	ngOnInit(): void {}

	next(details: { next_page_cursor: string }) {
		this.paginate.emit({ ...details, direction: 'next', prev_page_cursor: '' });
	}

	prev(details: { prev_page_cursor: string }) {
		this.paginate.emit({ ...details, direction: 'prev', next_page_cursor: '' });
	}
}
