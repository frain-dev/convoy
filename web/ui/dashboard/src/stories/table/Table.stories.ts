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
	template: `<table convoy-table>
                <thead convoy-table-head>
                    <th convoy-table-head-cell>Table head</th>
                    <th convoy-table-head-cell>Table head</th>
                    <th convoy-table-head-cell>Table head</th>
                    <th convoy-table-head-cell>Table head</th>
                </thead>
                <tbody>
                    <tr convoy-table-row [forDate]="forDate" *ngIf="forDate">
                        <td convoy-table-cell [forDate]="true">22nd Jan</td>
                        <td convoy-table-cell [forDate]="true"></td>
                        <td convoy-table-cell [forDate]="true"></td>
                        <td convoy-table-cell [forDate]="true"></td>
                    </tr>
                    <tr convoy-table-row [active]="active">
						<td convoy-table-cell>Table data</td>
						<td convoy-table-cell>Table data</td>
						<td convoy-table-cell>Table data</td>
						<td convoy-table-cell>Table data</td>
					</tr>
                    <tr convoy-table-row>
						<td convoy-table-cell>Table data</td>
						<td convoy-table-cell>Table data</td>
						<td convoy-table-cell>Table data</td>
						<td convoy-table-cell>Table data</td>
					</tr>
                </tbody>
            </table>`
});

export const Base = Template.bind({});
Base.args = {
	forDate: false,
	active: false
} as Partial<TableComponent>;

export const WithDate = Template.bind({});
WithDate.args = {
	forDate: true,
	active: true
} as Partial<TableComponent>;
