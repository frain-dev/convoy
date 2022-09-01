import { Story, Meta } from '@storybook/angular/types-6-0';
import { moduleMetadata } from '@storybook/angular';
import { TableComponent } from '../../app/components/table/table.component';
import { TableHeadComponent } from '../../app/components/table-head/table-head.component';
import { TableCellComponent } from '../../app/components/table-cell/table-cell.component';
import { TableHeadCellComponent } from '../../app/components/table-head-cell/table-head-cell.component';
import { TableRowComponent } from '../../app/components/table-row/table-row.component';

export default {
	title: 'Example/Table',
	component: TableComponent,
	subcomponents: [TableHeadComponent, TableCellComponent, TableHeadCellComponent, TableRowComponent],
	decorators: [
		moduleMetadata({
			imports: [TableComponent, TableHeadComponent, TableCellComponent, TableHeadCellComponent, TableRowComponent]
		})
	],
	argTypes: {
		class: {
			control: { type: 'text' }
		},
		forDate: {
			control: { type: 'boolean' }
		},
		active: {
			control: { type: 'boolean' }
		}
	}
} as Meta;

const Template: Story<TableComponent> = (args: TableComponent) => ({
	props: args,
	template: `<convoy-table>
                <convoy-table-head class="contents">
                    <convoy-table-head-cell class="contents">head</convoy-table-head-cell>
                    <convoy-table-head-cell class="contents">head</convoy-table-head-cell>
                    <convoy-table-head-cell class="contents">head</convoy-table-head-cell>
                    <convoy-table-head-cell class="contents">head</convoy-table-head-cell>
                </convoy-table-head>
                <tbody>
                    <convoy-table-row [forDate]="forDate" class="contents">
                        <convoy-table-cell class="contents" [forDate]="true">22nd Jan</convoy-table-cell>
                        <convoy-table-cell class="contents" [forDate]="true"></convoy-table-cell>
                        <convoy-table-cell class="contents" [forDate]="true"></convoy-table-cell>
                        <convoy-table-cell class="contents" [forDate]="true"></convoy-table-cell>
                    </convoy-table-row>
                    <convoy-table-row class="contents" [active]="active">
						<convoy-table-cell class="contents">data</convoy-table-cell>
						<convoy-table-cell class="contents">data</convoy-table-cell>
						<convoy-table-cell class="contents">data</convoy-table-cell>
						<convoy-table-cell class="contents">data</convoy-table-cell>
					</convoy-table-row>
                    <convoy-table-row class="contents">
						<convoy-table-cell class="contents">data</convoy-table-cell>
						<convoy-table-cell class="contents">data</convoy-table-cell>
						<convoy-table-cell class="contents">data</convoy-table-cell>
						<convoy-table-cell class="contents">data</convoy-table-cell>
					</convoy-table-row>
                </tbody>
            </convoy-table>`
});

export const Base = Template.bind({});
Base.args = {
	class: 'm-auto',
	forDate: true,
	active: true
} as Partial<TableComponent>;
