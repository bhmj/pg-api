-- Sample function to store file metadata
--
-- There are 4 standard fields in _data, all sent as text:
--   - "bucket"      : name of S3 bucket (from query)
--   - "filename"    : file name
--   - "fileext"     : file extension
--   - "filesize"    : file size (convertible to bigint)
--
-- All other multipart fields are passed as text fields in _data.
--
-- Return values:
--   One row table with optional path prefix and error message

CREATE OR REPLACE FUNCTION store_file_metadata(_data json)
 returns table (prefix text, error text) 
 LANGUAGE plpgsql
AS $function$
declare
    _bucket text;
    _prefix text;
    _new_id bigint;
begin
    _bucket = _data->>'bucket';

    if _bucket == 'special' then
      _prefix = 'special/'
    end if;

    -- example table.
    insert into file_metadata (bucket, file_name, file_ext, file_size, some_data)
    select 
        _bucket,
        _data->>'filename',
        _data->>'fileext',
        (_data->>'filesize')::bigint,
        (_data->>'some_data')::text    -- example field
    on conflict (bucket, file_name, file_ext) do update set
        file_size = excluded.file_size,
        some_data = excluded.some_data
    returning id into _new_id
    ;

    return query 
        select _prefix, '';
   
	exception when others then
	
	return query
        select '', sqlerrm;
end
$function$;
